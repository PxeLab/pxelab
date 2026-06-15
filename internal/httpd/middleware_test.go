package httpd

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
	t.Run("missing token returns 401", func(t *testing.T) {
		handler := AuthMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid token passes", func(t *testing.T) {
		called := false
		handler := AuthMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		r.Header.Set("Authorization", "Bearer secret123")
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if !called {
			t.Error("expected handler to be called")
		}
	})

	t.Run("wrong token returns 401", func(t *testing.T) {
		handler := AuthMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not be called")
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		r.Header.Set("Authorization", "Bearer wrongtoken")
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("public path exempted", func(t *testing.T) {
		called := false
		handler := AuthMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		paths := []string{"/health", "/api/v1/status", "/boot/ipxe.efi"}
		for _, p := range paths {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", p, nil)
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

	t.Run("empty token skips auth", func(t *testing.T) {
		called := false
		handler := AuthMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if !called {
			t.Error("handler should be called when no auth token is set")
		}
	})
}
