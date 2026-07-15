package api

import (
	"net/http"

	"github.com/pxelab/pxelab/internal/metrics"
)

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	snap := metrics.DefaultRegistry.Snapshot()
	OK(w, snap)
}
