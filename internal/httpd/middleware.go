package httpd

import (
	"net/http"
	"strings"
)

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

// publicAPIPaths 是不需要认证的 API 路径前缀
var publicAPIPaths = []string{
	"/api/v1/status",
	"/api/v1/settings",
	"/api/v1/interfaces",
	"/api/v1/events/stream",
	"/api/v1/logs/stream",
}

func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			// 非 API 路径（SPA、静态资源、启动文件等）无需认证
			if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
				next.ServeHTTP(w, r)
				return
			}
			// API 路径中的公开端点无需认证
			for _, p := range publicAPIPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
