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

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	// 压制 go-nfs 库内部杂音（如 NFSACL 100227 未注册）
	gonfs.Log.SetLevel(gonfs.FatalLevel)

	if err := os.MkdirAll(s.rootDir, 0755); err != nil {
		return fmt.Errorf("nfs: 创建根目录失败: %w", err)
	}

	fs := osfs.New(s.rootDir)

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

	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("nfs: 监听端口 %d 失败: %w", s.port, err)
	}
	s.listener = listener

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// 包装 Listener：拦截 NFSv4（prog=100003, vers>=4）并回复 PROG_MISMATCH，
	// 强制 Linux 客户端自动回退到 v3
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

	// 启动内嵌 rpcbind（Linux 客户端需通过 port 111 发现 NFS 端口）
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

type nfsHandler struct {
	fs        billy.Filesystem
	readOnly  bool
	allowNets []*net.IPNet
}

func (h *nfsHandler) Mount(ctx context.Context, conn net.Conn, req gonfs.MountRequest) (gonfs.MountStatus, billy.Filesystem, []gonfs.AuthFlavor) {
	if !h.isAllowed(conn.RemoteAddr()) {
		slog.Warn("NFS 挂载被拒绝", "service", "nfs", "client", conn.RemoteAddr().String())
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

// bufferedConn wraps a net.Conn and returns pre-read bytes first,
// then transparently reads from the underlying connection.
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

// versionAwareListener intercepts TCP connections to reject NFSv4+ RPC calls
// with PROG_MISMATCH, forcing clients to fall back to NFSv3.
type versionAwareListener struct {
	net.Listener
}

func (l *versionAwareListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		// Read TCP record marker (4 bytes) + RPC call header (20 bytes):
		//   xid(4) + msg_type(4) + rpcvers(4) + prog(4) + vers(4)
		header := make([]byte, 24)
		if _, err := io.ReadFull(conn, header); err != nil {
			conn.Close()
			continue
		}

		msgType := binary.BigEndian.Uint32(header[8:12])
		prog := binary.BigEndian.Uint32(header[16:20])
		vers := binary.BigEndian.Uint32(header[20:24])

		if msgType == 0 && prog == 100003 && vers >= 4 {
			xid := binary.BigEndian.Uint32(header[4:8])
			slog.Info("NFSv4 连接已拒绝（PROG_MISMATCH）", "service", "nfs", "xid", xid, "vers", vers)
			reply := buildProgMismatchReply(xid, 3, 3)
			marker := make([]byte, 4)
			binary.BigEndian.PutUint32(marker, uint32(len(reply))|(1<<31))
			conn.Write(append(marker, reply...))
			conn.Close()
			continue
		}

		return &bufferedConn{Conn: conn, buf: header}, nil
	}
}

func buildProgMismatchReply(xid uint32, low, high uint32) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, xid)       // xid
	binary.Write(&buf, binary.BigEndian, uint32(1))  // msg_type = REPLY
	binary.Write(&buf, binary.BigEndian, uint32(0))  // reply_stat = MSG_ACCEPTED
	binary.Write(&buf, binary.BigEndian, uint32(0))  // verf flavor = AUTH_NONE
	binary.Write(&buf, binary.BigEndian, uint32(0))  // verf length = 0
	binary.Write(&buf, binary.BigEndian, uint32(2))  // accept_stat = PROG_MISMATCH
	binary.Write(&buf, binary.BigEndian, low)         // low version
	binary.Write(&buf, binary.BigEndian, high)        // high version
	return buf.Bytes()
}
