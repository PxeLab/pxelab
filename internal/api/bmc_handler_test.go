package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/store"
)

func TestBMCImportCSV(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "raw CSV body",
			body: "10.0.0.1,623,admin,admin,ipmi\n10.0.0.2,443,root,root,redfish\n",
		},
		{
			name: "JSON wrapped csv field",
			body: `{"csv":"10.0.0.1,623,admin,admin,ipmi\n10.0.0.2,443,root,root,redfish\n"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewBMCHandler(store.NewMemory())
			req := httptest.NewRequest(http.MethodPost, "/api/bmc/configs/import", strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			h.ImportCSV(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			var out Response
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			data, ok := out.Data.(map[string]any)
			if !ok {
				t.Fatalf("unexpected data shape: %T", out.Data)
			}
			if data["success"] != float64(2) {
				t.Errorf("expected success=2, got %v", data["success"])
			}
			if data["failed"] != float64(0) {
				t.Errorf("expected failed=0, got %v", data["failed"])
			}
		})
	}
}

func TestBMCImportCSVInvalidLines(t *testing.T) {
	h := NewBMCHandler(store.NewMemory())
	// 第二行缺少字段（< 4），第三行 host 为空
	body := `{"csv":"10.0.0.1,623,admin,admin,ipmi\n10.0.0.2,623\n,623,admin,admin,ipmi\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/bmc/configs/import", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ImportCSV(w, req)

	var out Response
	if err := json.NewDecoder(w.Result().Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := out.Data.(map[string]any)
	if data["success"] != float64(1) {
		t.Errorf("expected success=1, got %v", data["success"])
	}
	if data["failed"] != float64(2) {
		t.Errorf("expected failed=2, got %v", data["failed"])
	}
}
