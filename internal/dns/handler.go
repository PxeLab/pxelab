package dns

import (
	"fmt"
	"log/slog"

	"github.com/miekg/dns"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
)

type Handler struct {
	upstream string
	client   *dns.Client
	eventBus *eventbus.Bus
}

func NewHandler(upstream string, bus *eventbus.Bus) *Handler {
	return &Handler{
		upstream: upstream,
		client:   &dns.Client{},
		eventBus: bus,
	}
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	if len(r.Question) == 0 {
		return
	}

	q := r.Question[0]
	slog.Debug("DNS 查询", "service", "DNS", "name", q.Name, "type", q.Qtype)

	if h.upstream != "" {
		resp, _, err := h.client.Exchange(r, h.upstream)
		if err != nil {
			slog.Error("DNS 转发失败", "service", "DNS", "error", err)
			m.Rcode = dns.RcodeServerFailure
		} else {
			m = resp
		}
	}

	h.eventBus.Publish("event", models.Event{
		Type:    models.EventDNS,
		Level:   models.EventInfo,
		Message: fmt.Sprintf("DNS 查询: %s", q.Name),
	})
	w.WriteMsg(m)
}
