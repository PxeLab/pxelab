package nfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/willscott/go-nfs-client/nfs/xdr"
)

const (
	pmapProgram   = 100000
	pmapVersion   = 2
	procNull      = 0
	procGetPort   = 3
	procDump      = 4
	protoTCP      = 6
	protoUDP      = 17
)

type portmapEntry struct {
	Program  uint32
	Version  uint32
	Protocol uint32
	Port     uint32
}

type rpcbindServer struct {
	port     int
	udpConn  *net.UDPConn
	tcpLn    net.Listener
	cancel   context.CancelFunc
	mu       sync.RWMutex
	entries  []portmapEntry
}

func newRPCBind(nfsPort int) *rpcbindServer {
	return &rpcbindServer{
		port: 111,
		entries: []portmapEntry{
			{100003, 3, protoTCP, uint32(nfsPort)}, // NFSv3 over TCP
			{100003, 3, protoUDP, uint32(nfsPort)}, // NFSv3 over UDP (same port, client fallback)
			{100005, 1, protoTCP, uint32(nfsPort)}, // MOUNT v1 over TCP
			{100005, 1, protoUDP, uint32(nfsPort)}, // MOUNT v1 over UDP
			{100005, 3, protoTCP, uint32(nfsPort)}, // MOUNT v3 over TCP
			{100005, 3, protoUDP, uint32(nfsPort)}, // MOUNT v3 over UDP
		},
	}
}

func (r *rpcbindServer) Start(ctx context.Context) error {
	// UDP listener
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", r.port))
	if err != nil {
		return fmt.Errorf("rpcbind: 地址解析失败: %w", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("rpcbind: 监听 UDP %d 失败: %w", r.port, err)
	}
	r.udpConn = conn

	// TCP listener (Linux mount.nfs 可能使用 TCP 查询 rpcbind)
	tcpLn, err := net.Listen("tcp", fmt.Sprintf(":%d", r.port))
	if err != nil {
		slog.Warn("rpcbind: TCP 监听失败", "service", "nfs", "port", r.port, "error", err)
	} else {
		r.tcpLn = tcpLn
	}

	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	go r.serveUDP(ctx)
	if r.tcpLn != nil {
		go r.serveTCP(ctx)
	}
	return nil
}

func (r *rpcbindServer) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	if r.udpConn != nil {
		r.udpConn.Close()
	}
	if r.tcpLn != nil {
		r.tcpLn.Close()
	}
}

func (r *rpcbindServer) serveUDP(ctx context.Context) {
	buf := make([]byte, 65536)
	slog.Info("rpcbind (UDP) 服务已启动", "service", "nfs", "port", r.port)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		r.udpConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := r.udpConn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("rpcbind UDP 读取错误", "service", "nfs", "error", err)
				continue
			}
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		go r.handleUDPPacket(addr, data)
	}
}

func (r *rpcbindServer) serveTCP(ctx context.Context) {
	slog.Info("rpcbind (TCP) 服务已启动", "service", "nfs", "port", r.port)
	for {
		conn, err := r.tcpLn.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("rpcbind TCP 接受失败", "service", "nfs", "error", err)
				continue
			}
		}
		go r.handleTCPConn(ctx, conn)
	}
}

func (r *rpcbindServer) handleTCPConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 65536)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		// TCP RPC 消息前有 4 字节 record marker，去掉它
		if len(data) < 4 {
			return
		}
		data = data[4:]
		resp, err := r.processRequest(data)
		if err != nil {
			slog.Warn("rpcbind TCP 处理失败", "service", "nfs", "error", err)
			return
		}
		// TCP 响应也要加 record marker
		marker := make([]byte, 4)
		binary.BigEndian.PutUint32(marker, uint32(len(resp))|(1<<31))
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write(append(marker, resp...)); err != nil {
			return
		}
	}
}

func (r *rpcbindServer) handleUDPPacket(addr *net.UDPAddr, data []byte) {
	resp, err := r.processRequest(data)
	if err != nil {
		slog.Info("rpcbind UDP 处理失败", "service", "nfs", "error", err)
		return
	}
	r.udpConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := r.udpConn.WriteToUDP(resp, addr); err != nil {
		slog.Info("rpcbind UDP 发送失败", "service", "nfs", "error", err)
	}
}

func (r *rpcbindServer) processRequest(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)

	// RPC Call header
	xid, err := readUint32(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 xid 失败: %w", err)
	}

	msgType, err := readUint32(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 msg_type 失败: %w", err)
	}
	if msgType != 0 { // CALL
		return nil, fmt.Errorf("不是 CALL 消息: %d", msgType)
	}

	rpcvers, err := readUint32(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 rpcvers 失败: %w", err)
	}
	_ = rpcvers

	prog, err := readUint32(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 prog 失败: %w", err)
	}
	if prog != pmapProgram {
		return nil, fmt.Errorf("不是 portmap 程序: %d", prog)
	}

	vers, err := readUint32(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 vers 失败: %w", err)
	}
	_ = vers

	proc, err := readUint32(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 proc 失败: %w", err)
	}

	// Skip auth credentials
	if err := skipAuth(reader); err != nil {
		return nil, fmt.Errorf("跳过认证失败: %w", err)
	}
	// Skip auth verifier
	if err := skipAuth(reader); err != nil {
		return nil, fmt.Errorf("跳过验证者失败: %w", err)
	}

	var resultBuf bytes.Buffer

	switch proc {
	case procNull:
		// No args, no result
	case procGetPort:
		var mapping pmap
		if err := xdr.Read(reader, &mapping); err != nil {
			return nil, fmt.Errorf("读取 pmap 失败: %w", err)
		}
		port := r.lookupPort(mapping.Program, mapping.Version, mapping.Protocol)
		// 如果指定协议没找到，尝试任意协议（部分客户端用 protocol=0 查询）
		if port == 0 && mapping.Protocol == 0 {
			port = r.lookupAnyProtocol(mapping.Program, mapping.Version)
		}
		slog.Info("rpcbind GETPORT", "service", "nfs", "prog", mapping.Program,
			"vers", mapping.Version, "prot", mapping.Protocol, "port", port)
		if err := xdr.Write(&resultBuf, port); err != nil {
			return nil, fmt.Errorf("写入端口失败: %w", err)
		}
	case procDump:
		r.mu.RLock()
		entries := r.entries
		r.mu.RUnlock()
		// Filter unique entries for dump
		seen := make(map[[4]uint32]bool)
		var filtered []portmapEntry
		for _, e := range entries {
			key := [4]uint32{e.Program, e.Version, e.Protocol, e.Port}
			if !seen[key] {
				seen[key] = true
				filtered = append(filtered, e)
			}
		}
		slog.Info("rpcbind DUMP", "service", "nfs", "count", len(filtered))
		if err := writePortmapList(&resultBuf, filtered); err != nil {
			return nil, fmt.Errorf("写入 dump 失败: %w", err)
		}
	default:
		return nil, fmt.Errorf("未知 procedure: %d", proc)
	}

	return buildRPCReply(xid, resultBuf.Bytes()), nil
}

func (r *rpcbindServer) lookupPort(prog, vers, prot uint32) uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		if e.Program == prog && e.Version == vers && e.Protocol == prot {
			return e.Port
		}
	}
	return 0
}

func (r *rpcbindServer) lookupAnyProtocol(prog, vers uint32) uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		if e.Program == prog && e.Version == vers {
			return e.Port
		}
	}
	return 0
}

func readUint32(r io.Reader) (uint32, error) {
	var v uint32
	if err := binary.Read(r, binary.BigEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func skipAuth(r io.Reader) error {
	flavor, err := readUint32(r)
	if err != nil {
		return err
	}
	_ = flavor
	length, err := readUint32(r)
	if err != nil {
		return err
	}
	// Skip body + padding to 4 bytes
	padded := (length + 3) & ^uint32(3)
	_, err = io.CopyN(io.Discard, r, int64(padded))
	return err
}

func buildRPCReply(xid uint32, body []byte) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, xid)       // xid
	binary.Write(&buf, binary.BigEndian, uint32(1))  // msg_type = REPLY
	binary.Write(&buf, binary.BigEndian, uint32(0))  // reply_stat = MSG_ACCEPTED
	// Auth verifier (none)
	binary.Write(&buf, binary.BigEndian, uint32(0))  // flavor = AUTH_NONE
	binary.Write(&buf, binary.BigEndian, uint32(0))  // length = 0
	binary.Write(&buf, binary.BigEndian, uint32(0))  // accept_stat = SUCCESS
	buf.Write(body)
	return buf.Bytes()
}

type pmap struct {
	Program  uint32
	Version  uint32
	Protocol uint32
	Port     uint32
}

func writePortmapList(w io.Writer, entries []portmapEntry) error {
	// Write as XDR-encoded array: length + elements
	if err := xdr.Write(w, uint32(len(entries))); err != nil {
		return err
	}
	for _, e := range entries {
		if err := xdr.Write(w, e); err != nil {
			return err
		}
	}
	return nil
}

// startRPCBind starts the rpcbind UDP listener on port 111.
// Returns nil if port 111 is unavailable (e.g., not running as admin).
func startRPCBind(ctx context.Context, nfsPort int) *rpcbindServer {
	srv := newRPCBind(nfsPort)
	if err := srv.Start(ctx); err != nil {
		slog.Warn("rpcbind 无法启动（可能需要管理员权限）", "service", "nfs", "error", err)
		return nil
	}
	return srv
}
