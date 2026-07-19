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
	"path"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gonfs "github.com/willscott/go-nfs"
	"github.com/willscott/go-nfs/helpers"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/metrics"
)

type mountEntry struct {
	label      string
	exportPath string
	localDir   string
	readOnly   bool
	allowNets  []*net.IPNet
	fs         billy.Filesystem
}

type Server struct {
	name        string
	port        int
	mountPoints []config.NFSMountPoint
	mu          sync.RWMutex
	listener    net.Listener
	rpcbind     *rpcbindServer
	cancel      context.CancelFunc
	tracker     *ConnectionTracker
}

func NewServer(port int, mountPoints []config.NFSMountPoint) *Server {
	return &Server{
		name:        "NFS",
		port:        port,
		mountPoints: mountPoints,
		tracker:     NewConnectionTracker(),
	}
}

func (s *Server) SetMountPoints(mps []config.NFSMountPoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mountPoints = mps
}

func (s *Server) Name() string { return s.name }

// GetConnectionStats returns connection statistics for all mount points
func (s *Server) GetConnectionStats() map[string]MountStats {
	return s.tracker.GetStats()
}

func parseAllowIPs(ips []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, raw := range ips {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(raw)
		if err != nil {
			ip := net.ParseIP(raw)
			if ip == nil {
				slog.Warn("NFS 允许列表解析失败，跳过", "service", "nfs", "value", raw)
				continue
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			cidr = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		}
		nets = append(nets, cidr)
	}
	return nets
}

func (s *Server) Start(ctx context.Context) error {
	gonfs.Log.SetLevel(gonfs.FatalLevel)

	s.mu.RLock()
	mps := s.mountPoints
	s.mu.RUnlock()

	var entries []mountEntry
	seenExports := make(map[string]string) // exportPath -> label (for duplicate detection)
	for _, mp := range mps {
		localDir := mp.LocalDir
		if localDir == "" {
			continue
		}
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("nfs: 创建目录失败 %s: %w", localDir, err)
		}

		rawFS := osfs.New(localDir)
		var fs billy.Filesystem = rawFS
		if mp.ReadOnly {
			fs = &readOnlyFS{Filesystem: rawFS}
		}

		exportPath := mp.ExportPath
		if exportPath == "" {
			exportPath = "/"
		}
		exportPath = path.Clean(exportPath)

		if prevLabel, exists := seenExports[exportPath]; exists {
			slog.Warn("NFS 重复导出路径，后者将被忽略", "service", "nfs",
				"export", exportPath, "previous_label", prevLabel, "current_label", mp.Label)
			continue
		}
		seenExports[exportPath] = mp.Label

		entry := mountEntry{
			label:      mp.Label,
			exportPath: exportPath,
			localDir:   localDir,
			readOnly:   mp.ReadOnly,
			allowNets:  parseAllowIPs(mp.AllowIPs),
			fs:         fs,
		}
		entries = append(entries, entry)

		if len(entry.allowNets) > 0 {
			allowList := make([]string, len(entry.allowNets))
			for i, n := range entry.allowNets {
				allowList[i] = n.String()
			}
			slog.Info("NFS 挂载点 IP 允许列表", "service", "nfs", "export", exportPath, "allow", allowList)
		}
		slog.Info("NFS 挂载点已注册", "service", "nfs", "export", exportPath, "local", localDir, "read_only", mp.ReadOnly)
	}

	handler := &nfsHandler{entries: entries, tracker: s.tracker}
	cachingHandler := helpers.NewCachingHandler(handler, 1000)

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
		slog.Info("NFS 服务已启动", "service", "NFS", "port", s.port, "mount_points", len(entries))
		if err := gonfs.Serve(versionListener, cachingHandler); err != nil {
			select {
			case <-ctx.Done():
			default:
				slog.Error("NFS Serve 异常退出", "service", "NFS", "error", err)
			}
		}
	}()

	s.rpcbind = startRPCBind(ctx, s.port)

	// Start connection tracker
	s.tracker.Start()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("NFS 服务关闭", "service", "NFS")
	// Stop connection tracker
	s.tracker.Stop()
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
	entries []mountEntry
	tracker *ConnectionTracker
}

func (h *nfsHandler) findEntry(dirpath string) *mountEntry {
	cleaned := path.Clean(dirpath)
	for i := range h.entries {
		if h.entries[i].exportPath == cleaned {
			return &h.entries[i]
		}
	}
	return nil
}

func (h *nfsHandler) findEntryByFS(fs billy.Filesystem) *mountEntry {
	for i := range h.entries {
		if h.entries[i].fs == fs {
			return &h.entries[i]
		}
	}
	return nil
}

func (h *nfsHandler) Mount(ctx context.Context, conn net.Conn, req gonfs.MountRequest) (gonfs.MountStatus, billy.Filesystem, []gonfs.AuthFlavor) {
	nfsMetrics.RecordRequest()
	clientAddr := conn.RemoteAddr().String()
	dirpath := strings.TrimSpace(string(req.Dirpath))
	if dirpath == "" {
		dirpath = "/"
	}

	entry := h.findEntry(dirpath)
	if entry == nil {
		nfsMetrics.RecordRejected()
		slog.Warn("NFS 挂载失败（导出路径不存在）", "service", "nfs", "client", clientAddr, "path", dirpath)
		return gonfs.MountStatusErrNoEnt, nil, nil
	}

	if !isAllowed(conn.RemoteAddr(), entry.allowNets) {
		nfsMetrics.RecordRejected()
		if len(entry.allowNets) > 0 {
			allowList := make([]string, len(entry.allowNets))
			for i, n := range entry.allowNets {
				allowList[i] = n.String()
			}
			slog.Warn("NFS 挂载被拒绝（不在允许列表）", "service", "nfs", "client", clientAddr, "export", entry.exportPath, "allow", allowList)
		} else {
			slog.Warn("NFS 挂载被拒绝", "service", "nfs", "client", clientAddr, "export", entry.exportPath)
		}
		return gonfs.MountStatusErrAcces, nil, nil
	}

	slog.Info("NFS 挂载成功", "service", "nfs", "client", clientAddr, "export", entry.exportPath, "local", entry.localDir)

	// Record connection in tracker
	if h.tracker != nil {
		h.tracker.RecordMount(entry.exportPath, clientAddr)
	}

	return gonfs.MountStatusOk, entry.fs, []gonfs.AuthFlavor{gonfs.AuthFlavorNull}
}

func isAllowed(addr net.Addr, allowNets []*net.IPNet) bool {
	if len(allowNets) == 0 {
		return true
	}
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		slog.Warn("NFS 客户端地址类型异常，拒绝访问", "service", "nfs", "client", addr.String(), "type", addr.Network())
		return false
	}
	for _, n := range allowNets {
		if n.Contains(tcpAddr.IP) {
			return true
		}
	}
	return false
}

func (h *nfsHandler) Change(fs billy.Filesystem) billy.Change {
	entry := h.findEntryByFS(fs)
	if entry != nil && entry.readOnly {
		return nil
	}
	if c, ok := fs.(billy.Change); ok {
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
