package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"strings"

	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/session"
)

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]int
	stop     chan struct{}
}

func newLoginRateLimiter() *loginRateLimiter {
	l := &loginRateLimiter{
		attempts: make(map[string]int),
		stop:     make(chan struct{}),
	}
	go l.cleanupLoop()
	return l
}

func (l *loginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			l.attempts = make(map[string]int)
			l.mu.Unlock()
		case <-l.stop:
			return
		}
	}
}

func (l *loginRateLimiter) record(ip string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[ip]++
	return l.attempts[ip]
}

func (l *loginRateLimiter) clear(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

type AuthHandler struct {
	cfg      *config.Config
	sessions *session.Store
	limiter  *loginRateLimiter
}

func NewAuthHandler(cfg *config.Config, sessions *session.Store) *AuthHandler {
	return &AuthHandler{cfg: cfg, sessions: sessions, limiter: newLoginRateLimiter()}
}

type loginRequest struct {
	Token string `json:"token"`
}

type loginResponse struct {
	SessionToken string `json:"session_token"`
	ExpiresAt    string `json:"expires_at"`
	TokenSet     bool   `json:"token_set"`
}

// Login 验证 master token 并创建 session
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// 登录频率限制：同一 IP 5 次失败后加入工延迟
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	n := h.limiter.record(ip)
	if n > 5 {
		time.Sleep(time.Duration(n-5) * 200 * time.Millisecond)
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Token == "" {
		Error(w, http.StatusBadRequest, "token 不能为空")
		return
	}

	// 通过 token hash 验证
	hsh := sha256.Sum256([]byte(req.Token))
	expected, err := hex.DecodeString(h.cfg.Auth.TokenHash)
	if err != nil || subtle.ConstantTimeCompare(hsh[:], expected) != 1 {
		// 兼容旧版本：明文 token 对比（token hash 为空时）
		if h.cfg.Auth.TokenHash == "" && req.Token == h.cfg.Auth.Token {
			// 升级存储 hash
			h.cfg.Auth.TokenHash = hex.EncodeToString(hsh[:])
			slog.Info("已升级 token 存储为 SHA-256 hash")
		} else {
			Error(w, http.StatusUnauthorized, "token 无效")
			return
		}
	}

	h.limiter.clear(ip)
	session := h.sessions.Create()
	slog.Info("用户登录成功", "remote", r.RemoteAddr)

	OK(w, loginResponse{
		SessionToken: session.Token,
		ExpiresAt:    session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		TokenSet:     true,
	})
}

// Logout 销毁 session
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		h.sessions.Revoke(token)
	}
	OK(w, map[string]string{"status": "logged_out"})
}

// Session 检查当前 session 是否有效
func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if h.sessions.IsValid(token) {
			OK(w, map[string]any{"valid": true})
			return
		}
	}
	OK(w, map[string]any{"valid": false})
}
