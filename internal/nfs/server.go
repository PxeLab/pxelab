package nfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gonfs "github.com/willscott/go-nfs"
	"github.com/willscott/go-nfs/helpers"
	"github.com/pxelab/pxelab/internal/metrics"
)

type Server struct {
	name       string
	port       int
	rootDir    string
	readOnly   bool
	allowIPs   []string
	listener   net.Listener
	rpcbind    *rpcbindServer
	cancel     context.CancelFunc
}

func NewServer(port int, rootDir string, readOnly bool, allowIPs []string) *Server {
	return &Server{
		name:     "NFS",
		port:     port,
		rootDir:  rootDir,
		readOnly: readOnly,
		allowIPs: allowIPs,
	}
}

func (s *Server) SetAllowIPs(ips []string) {
	s.allowIPs = ips
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	gonfs.Log.SetLevel(gonfs.FatalLevel)

	if err := os.MkdirAll(s.rootDir, 0755); err != nil {
		return fmt.Errorf("nfs: 创建根目录失败: %w", err)
	}

	rawFS := osfs.New(s.rootDir)

	var fs billy.Filesystem = rawFS
	if s.readOnly {
		fs = &readOnlyFS{Filesystem: rawFS}
	}

	var allowNets []*net.IPNet
	for _, s := range s.allowIPs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(s)
		if err != nil {
			ip := net.ParseIP(s)
			if ip == nil {
				slog.Warn("NFS 允许列表解析失败，跳过", "service", "nfs", "value", s)
				continue
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			cidr = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		}
		allowNets = append(allowNets, cidr)
	}

	handler := &nfsHandler{fs: fs, readOnly: s.readOnly, allowNets: allowNets}
	cachingHandler := helpers.NewCachingHandler(handler, 1000)

	if len(allowNets) > 0 {
		allowList := make([]string, len(allowNets))
		for i, n := range allowNets {
			allowList[i] = n.String()
		}
		slog.Info("NFS IP 允许列表", "service", "nfs", "allow", allowList)
	}

	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("nfs: 监听端口 %d 失败: %w", s.port, err)
	}
	s.listener = listener

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	versionListener := &versionAwareListener{Listener: listener}

	go func() {
		slog.Info("NFS 服务已启动", "service", "NFS", "port", s.port, "root", s.rootDir, "read_only", s.readOnly)
		if err := gonfs.Serve(versionListener, cachingHandler); err != nil {
			select {
			case <-ctx.Done():
			default:
				slog.Error("NFS Serve 异常退出", "service", "NFS", "error", err)
			}
		}
	}()

	s.rpcbind = startRPCBind(ctx, s.port)

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("NFS 服务关闭", "service", "NFS")
	if s.rpcbind != nil {
		s.rpcbind.Stop()
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

var nfsMetrics = metrics.DefaultRegistry.GetOrCreate("nfs")

type nfsHandler struct {
	fs        billy.Filesystem
	readOnly  bool
	allowNets []*net.IPNet
}

func (h *nfsHandler) Mount(ctx context.Context, conn net.Conn, req gonfs.MountRequest) (gonfs.MountStatus, billy.Filesystem, []gonfs.AuthFlavor) {
	nfsMetrics.RecordRequest()
	clientAddr := conn.RemoteAddr().String()
	if !h.isAllowed(conn.RemoteAddr()) {
		nfsMetrics.RecordRejected()
		if len(h.allowNets) > 0 {
			allowList := make([]string, len(h.allowNets))
			for i, n := range h.allowNets {
				allowList[i] = n.String()
			}
			slog.Warn("NFS 挂载被拒绝（不在允许列表）", "service", "nfs", "client", clientAddr, "allow", allowList)
		} else {
			slog.Warn("NFS 挂载被拒绝", "service", "nfs", "client", clientAddr)
		}
		return gonfs.MountStatusErrAcces, nil, nil
	}
	return gonfs.MountStatusOk, h.fs, []gonfs.AuthFlavor{gonfs.AuthFlavorNull}
}

func (h *nfsHandler) isAllowed(addr net.Addr) bool {
	if len(h.allowNets) == 0 {
		return true
	}
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		slog.Warn("NFS 客户端地址类型异常，拒绝访问", "service", "nfs", "client", addr.String(), "type", addr.Network())
		return false
	}
	for _, n := range h.allowNets {
		if n.Contains(tcpAddr.IP) {
			return true
		}
	}
	return false
}

func (h *nfsHandler) Change(fs billy.Filesystem) billy.Change {
	if h.readOnly {
		return nil
	}
	if c, ok := h.fs.(billy.Change); ok {
		return c
	}
	return nil
}

func (h *nfsHandler) FSStat(ctx context.Context, f billy.Filesystem, s *gonfs.FSStat) error {
	s.TotalSize = 1 << 60
	s.FreeSize = 1 << 59
	s.AvailableSize = 1 << 59
	s.TotalFiles = 1 << 30
	s.FreeFiles = 1 << 29
	s.AvailableFiles = 1 << 29
	return nil
}

func (h *nfsHandler) ToHandle(f billy.Filesystem, s []string) []byte {
	return []byte{}
}

func (h *nfsHandler) FromHandle([]byte) (billy.Filesystem, []string, error) {
	return nil, []string{}, nil
}

func (h *nfsHandler) InvalidateHandle(billy.Filesystem, []byte) error {
	return nil
}

func (h *nfsHandler) HandleLimit() int {
	return -1
}

type readOnlyFS struct {
	billy.Filesystem
}

func (readOnlyFS) Capabilities() billy.Capability {
	return billy.ReadCapability | billy.SeekCapability
}

type bufferedConn struct {
	net.Conn
	buf    []byte
	offset int
}

func (b *bufferedConn) Read(p []byte) (int, error) {
	if b.offset < len(b.buf) {
		n := copy(p, b.buf[b.offset:])
		b.offset += n
		return n, nil
	}
	return b.Conn.Read(p)
}

type versionAwareListener struct {
	net.Listener
}

func (l *versionAwareListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		header := make([]byte, 28)
		if _, err := io.ReadFull(conn, header); err != nil {
			conn.Close()
			continue
		}

		msgType := binary.BigEndian.Uint32(header[8:12])
		prog := binary.BigEndian.Uint32(header[16:20])
		vers := binary.BigEndian.Uint32(header[20:24])
		proc := binary.BigEndian.Uint32(header[24:28])
		xid := binary.BigEndian.Uint32(header[4:8])

		if msgType != 0 {
			conn.Close()
			continue
		}

		if prog == 100003 && vers >= 4 {
			slog.Info("NFSv4 连接已拒绝（PROG_MISMATCH）", "service", "nfs", "xid", xid, "vers", vers)
			reply := buildProgMismatchReply(xid, 3, 3)
			sendRPCReply(conn, reply)
			conn.Close()
			continue
		}

		if prog == 100005 && proc == 4 {
			slog.Debug("MOUNT DUMP 请求", "service", "nfs", "xid", xid)
			reply := buildMountDumpReply(xid)
			sendRPCReply(conn, reply)
			conn.Close()
			continue
		}

		if prog == 100005 && proc == 5 {
			slog.Debug("MOUNT EXPORT 请求", "service", "nfs", "xid", xid)
			reply := buildMountExportReply(xid)
			sendRPCReply(conn, reply)
			conn.Close()
			continue
		}

		return &bufferedConn{Conn: conn, buf: header}, nil
	}
}

func sendRPCReply(conn net.Conn, body []byte) {
	marker := make([]byte, 4)
	binary.BigEndian.PutUint32(marker, uint32(len(body))|(1<<31))
	conn.Write(append(marker, body...))
}

func buildRPCReplyHeader(xid uint32, acceptStat uint32) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, xid)
	binary.Write(&buf, binary.BigEndian, uint32(1))  // msg_type = REPLY
	binary.Write(&buf, binary.BigEndian, uint32(0))  // reply_stat = MSG_ACCEPTED
	binary.Write(&buf, binary.BigEndian, uint32(0))  // verf_flavor = AUTH_NONE
	binary.Write(&buf, binary.BigEndian, uint32(0))  // verf length = 0
	binary.Write(&buf, binary.BigEndian, acceptStat)
	return buf.Bytes()
}

func buildProgMismatchReply(xid uint32, low, high uint32) []byte {
	header := buildRPCReplyHeader(xid, 2) // PROG_MISMATCH
	var buf bytes.Buffer
	buf.Write(header)
	binary.Write(&buf, binary.BigEndian, low)
	binary.Write(&buf, binary.BigEndian, high)
	return buf.Bytes()
}

func buildMountExportReply(xid uint32) []byte {
	header := buildRPCReplyHeader(xid, 0) // SUCCESS

	var body bytes.Buffer

	body.Write([]byte{0, 0, 0, 1}) // pointer present (1 = non-null)

	dir := "/"
	body.Write([]byte{0, 0, 0, 1}) // string length = 1
	body.Write([]byte{dir[0]})     // "/"
	body.Write([]byte{0, 0, 0})    // padding to 4 bytes

	body.Write([]byte{0, 0, 0, 0}) // groups length = 0 (empty)

	body.Write([]byte{0, 0, 0, 0}) // NULL pointer (end of list)

	var buf bytes.Buffer
	buf.Write(header)
	buf.Write(body.Bytes())
	return buf.Bytes()
}

func buildMountDumpReply(xid uint32) []byte {
	header := buildRPCReplyHeader(xid, 0) // SUCCESS

	var body bytes.Buffer
	body.Write([]byte{0, 0, 0, 0}) // NULL pointer (empty list - no active mounts)

	var buf bytes.Buffer
	buf.Write(header)
	buf.Write(body.Bytes())
	return buf.Bytes()
}
