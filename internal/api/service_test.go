package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
)

// TestPublishServiceFailure 服务启停失败应发布到事件总线（事件页可见），
// 且 nil bus / nil err 时安全。
func TestPublishServiceFailure(t *testing.T) {
	bus := eventbus.New()
	var got []models.Event
	bus.Subscribe("event", func(e eventbus.Event) {
		if ev, ok := e.Payload.(models.Event); ok {
			got = append(got, ev)
		}
	})

	publishServiceFailure(bus, "启动", "smb", errors.New("listen tcp :445: bind: Only one usage"))
	if len(got) != 1 {
		t.Fatalf("应发布 1 个事件, got %d", len(got))
	}
	if got[0].Type != models.EventService {
		t.Fatalf("type = %q, want %q", got[0].Type, models.EventService)
	}
	if got[0].Level != models.EventError {
		t.Fatalf("level = %q, want %q", got[0].Level, models.EventError)
	}
	for _, want := range []string{"启动", "smb", "445"} {
		if !strings.Contains(got[0].Message, want) {
			t.Fatalf("message 应包含 %q: %s", want, got[0].Message)
		}
	}

	// 防御：nil bus / nil err 不应 panic，也不应额外发布
	publishServiceFailure(nil, "启动", "smb", errors.New("x"))
	publishServiceFailure(bus, "启动", "smb", nil)
	if len(got) != 1 {
		t.Fatalf("不应额外发布, got %d", len(got))
	}
}
