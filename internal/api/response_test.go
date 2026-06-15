package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONResponses(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		w := httptest.NewRecorder()
		OK(w, map[string]string{"hello": "world"})

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		var body Response
		json.NewDecoder(resp.Body).Decode(&body)
		if !body.Success {
			t.Error("expected success=true")
		}
	})

	t.Run("Created", func(t *testing.T) {
		w := httptest.NewRecorder()
		Created(w, "created")

		resp := w.Result()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected 201, got %d", resp.StatusCode)
		}
	})

	t.Run("Error", func(t *testing.T) {
		w := httptest.NewRecorder()
		Error(w, http.StatusBadRequest, "bad request")

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}

		var body Response
		json.NewDecoder(resp.Body).Decode(&body)
		if body.Success {
			t.Error("expected success=false")
		}
		if body.Error != "bad request" {
			t.Errorf("expected error='bad request', got %s", body.Error)
		}
	})
}
