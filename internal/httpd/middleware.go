package httpd

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/session"
)

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// slogMiddleware logs HTTP requests through slog with service=HTTP.
func slogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wr := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wr, r)

		attrs := []any{
			"service", "HTTP",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wr.status,
			"size", wr.size,
			"duration", time.Since(start).String(),
		}
		// Include client IP and MAC when present in query
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			attrs = append(attrs, "remote", host)
		}
		if mac := r.URL.Query().Get("mac"); mac != "" {
			attrs = append(attrs, "mac", mac)
		}

		slog.Info("HTTP 请求", attrs...)
	})
}

// publicAPIPaths 定义无需认证的 API 路径前缀。
// 与 api/handler.go 中的路由注册保持同步。
var publicAPIPaths = []string{
	"/api/v1/status",
	"/api/v1/events/stream",
	"/api/v1/logs/stream",
	"/api/v1/auth/",
		"/api/v1/netboot/task/by-mac/",
		"/api/v1/netboot/answer/",
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// needsAuth 检查当前监听地址是否要求远程认证
func needsAuth(cfg *config.Config) bool {
	addr := cfg.Global.ListenAddr
	if addr == "" {
		return false // 默认本地模式，无需认证
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return !(host == "127.0.0.1" || host == "::1" || host == "localhost")
}

// isLocalRequest 判断请求是否来自本机
func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// isPublicPath 判断路径是否无需认证
func isPublicPath(path string) bool {
	for _, p := range publicAPIPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return strings.HasPrefix(path, "/boot/") ||
		strings.HasPrefix(path, "/netboot/") ||
		path == "/health" ||
		path == "/login"
}

// AuthMiddleware 返回基于 session 和 token hash 的认证中间件。
// 行为：
//   - 127.0.0.1 模式 → 所有请求免认证
//   - 非 127.0.0.1 模式且请求来自本机 → 免认证
//   - 公共路径（/boot/, /netboot/, /api/v1/auth/, /health 等）→ 免认证
//   - API 路径 → 校验 Bearer token（session 或 master token hash）
//   - 非 API 路径（SPA）→ 放行（前端自行处理登录页跳转）
func AuthMiddleware(cfg *config.Config, sessions *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 仅监听非 127.0.0.1 时才考虑认证
			if !needsAuth(cfg) || isLocalRequest(r) || isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// API 路径需要认证
			if strings.HasPrefix(r.URL.Path, "/api/v1/") {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token := strings.TrimPrefix(authHeader, "Bearer ")

					// 校验 session token
					if sessions.IsValid(token) {
						next.ServeHTTP(w, r)
						return
					}

					// 校验 master token hash（兼容脚本直接使用 master token）
					if cfg.Auth.TokenHash != "" {
						h := sha256.Sum256([]byte(token))
						expected, err := hex.DecodeString(cfg.Auth.TokenHash)
						if err == nil && subtle.ConstantTimeCompare(h[:], expected) == 1 {
							next.ServeHTTP(w, r)
							return
						}
					}
				}

				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// SPA 等非 API 路径放行（前端控制登录页跳转）
			next.ServeHTTP(w, r)
		})
	}
}
