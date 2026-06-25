package httpd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/session"
)

func TestCORSMiddleware(t *testing.T) {
	t.Run("sets CORS headers", func(t *testing.T) {
		handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/test", nil)
		handler.ServeHTTP(w, r)

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("expected CORS origin *")
		}
	})

	t.Run("handles OPTIONS preflight", func(t *testing.T) {
		handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called for OPTIONS")
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("OPTIONS", "/test", nil)
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware(t *testing.T) {
	// Helper to create middleware with localhost config
	newMiddleware := func(cfg *config.Config, sessions *session.Store) func(http.Handler) http.Handler {
		return AuthMiddleware(cfg, sessions)
	}

	t.Run("localhost mode allows all", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Global.ListenAddr = "127.0.0.1:8080"
		sessions := session.NewStore(0)

		handler := newMiddleware(cfg, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 in localhost mode, got %d", w.Code)
		}
	})

	t.Run("remote mode missing token returns 401", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Global.ListenAddr = "0.0.0.0:8080"
		sessions := session.NewStore(0)

		handler := newMiddleware(cfg, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		r.RemoteAddr = "192.168.1.100:12345"
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid session token passes", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Global.ListenAddr = "0.0.0.0:8080"
		sessions := session.NewStore(0)
		s := sessions.Create()

		called := false
		handler := newMiddleware(cfg, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		r.RemoteAddr = "192.168.1.100:12345"
		r.Header.Set("Authorization", "Bearer "+s.Token)
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if !called {
			t.Error("expected handler to be called")
		}
	})

	t.Run("public path exempted", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Global.ListenAddr = "0.0.0.0:8080"
		sessions := session.NewStore(0)

		called := false
		handler := newMiddleware(cfg, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		paths := []string{"/health", "/api/v1/status", "/boot/ipxe.efi", "/api/v1/auth/login"}
		for _, p := range paths {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", p, nil)
			r.RemoteAddr = "192.168.1.100:12345"
			handler.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("path %s: expected 200, got %d", p, w.Code)
			}
			if !called {
				t.Errorf("path %s: handler should be called", p)
			}
			called = false
		}
	})

	t.Run("SPA paths are served without auth", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Global.ListenAddr = "0.0.0.0:8080"
		sessions := session.NewStore(0)

		called := false
		handler := newMiddleware(cfg, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/settings", nil)
		r.RemoteAddr = "192.168.1.100:12345"
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("SPA path: expected 200, got %d", w.Code)
		}
		if !called {
			t.Error("SPA handler should be called even without auth")
		}
	})
}
