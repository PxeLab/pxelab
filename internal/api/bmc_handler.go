package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/bmc"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type BMCHandler struct {
	store store.Interface
}

func NewBMCHandler(st store.Interface) *BMCHandler {
	return &BMCHandler{store: st}
}

// bmcConfigRequest is used for create/update to accept password (models.BMCConfig.Password
// is tagged with json:"-" so it cannot be used for decoding).
type bmcConfigRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Protocol string `json:"protocol"`
	Vendor   string `json:"vendor"`
	Model    string `json:"model"`
	Serial   string `json:"serial"`
	MAC      string `json:"mac"`
	Name     string `json:"name"`
	BootMode string `json:"boot_mode"`
}

// bmcConfigResponse wraps the model so that has_password is exposed while
// password is kept hidden (models.BMCConfig.Password has json:"-").
type bmcConfigResponse struct {
	models.BMCConfig
	HasPassword bool `json:"has_password"`
}

func toBMCConfigResponse(cfg models.BMCConfig) bmcConfigResponse {
	return bmcConfigResponse{
		BMCConfig:   cfg,
		HasPassword: cfg.Password != "",
	}
}

// convertRequestToModel copies non-zero fields from the request struct to a new BMCConfig.
// It does NOT set ID or timestamps -- the caller is responsible for that.
func convertRequestToModel(req bmcConfigRequest) models.BMCConfig {
	port := req.Port
	if port == 0 {
		port = 623
	}
	protocol := req.Protocol
	if protocol == "" {
		protocol = "ipmi"
	}
	return models.BMCConfig{
		Host:     req.Host,
		Port:     port,
		Username: req.Username,
		Password: req.Password,
		Protocol: protocol,
		Vendor:   req.Vendor,
		Model:    req.Model,
		Serial:   req.Serial,
		MAC:      req.MAC,
		Name:     req.Name,
		BootMode: req.BootMode,
	}
}

// ────────────────────────────── CRUD ──────────────────────────────

// List returns all BMC configs.
func (h *BMCHandler) List(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListBMCConfigs(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]bmcConfigResponse, len(configs))
	for i, cfg := range configs {
		resp[i] = toBMCConfigResponse(cfg)
	}
	OK(w, resp)
}

// Get returns a single BMC config. The password is always masked and
// has_password indicates whether a password is stored.
func (h *BMCHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	cfg, err := h.store.GetBMCConfig(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "BMC config not found")
		return
	}
	OK(w, toBMCConfigResponse(*cfg))
}

// Create adds a new BMC config. Host, username and password are required.
func (h *BMCHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req bmcConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Host == "" || req.Username == "" || req.Password == "" {
		Error(w, http.StatusBadRequest, "host, username and password are required")
		return
	}

	cfg := convertRequestToModel(req)
	if err := h.store.CreateBMCConfig(r.Context(), &cfg); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Created(w, toBMCConfigResponse(cfg))
}

// Update modifies an existing BMC config. If password is empty in the request
// the previously stored password is preserved.
func (h *BMCHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	existing, err := h.store.GetBMCConfig(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "BMC config not found")
		return
	}

	var req bmcConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Keep existing password when the client sends an empty one.
	if req.Password == "" {
		req.Password = existing.Password
	}

	cfg := convertRequestToModel(req)
	cfg.ID = id

	// Preserve existing fields when the client omits them.
	if cfg.Host == "" {
		cfg.Host = existing.Host
	}
	if cfg.Username == "" {
		cfg.Username = existing.Username
	}
	if cfg.Vendor == "" {
		cfg.Vendor = existing.Vendor
	}
	if cfg.Model == "" {
		cfg.Model = existing.Model
	}
	if cfg.Serial == "" {
		cfg.Serial = existing.Serial
	}
	if cfg.MAC == "" {
		cfg.MAC = existing.MAC
	}
	if cfg.Name == "" {
		cfg.Name = existing.Name
	}
	if cfg.BootMode == "" {
		cfg.BootMode = existing.BootMode
	}

	if err := h.store.UpdateBMCConfig(r.Context(), &cfg); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, toBMCConfigResponse(cfg))
}

// Delete removes a BMC config by ID.
func (h *BMCHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid ID")
		return
	}
	if err := h.store.DeleteBMCConfig(r.Context(), id); err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ────────────────────────────── CSV Import ──────────────────────────────

// ImportCSV creates BMC configs from a CSV body.
// Expected format per line: host,port,username,password,protocol
// Returns success and failed counts.
func (h *BMCHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	var (
		success int
		failed  int
	)

	reader := csv.NewReader(r.Body)
	reader.FieldsPerRecord = -1 // allow variable number of fields
	reader.TrimLeadingSpace = true

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			failed++
			continue
		}
		if len(record) < 4 {
			failed++
			continue
		}

		host := strings.TrimSpace(record[0])
		username := strings.TrimSpace(record[2])
		password := strings.TrimSpace(record[3])

		if host == "" || username == "" || password == "" {
			failed++
			continue
		}

		port := 623
		if len(record) > 1 {
			if p, err := strconv.Atoi(strings.TrimSpace(record[1])); err == nil && p > 0 {
				port = p
			}
		}

		protocol := "ipmi"
		if len(record) > 4 && strings.TrimSpace(record[4]) != "" {
			protocol = strings.TrimSpace(record[4])
		}

		cfg := &models.BMCConfig{
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
			Protocol: protocol,
		}

		if err := h.store.CreateBMCConfig(r.Context(), cfg); err != nil {
			failed++
			continue
		}
		success++
	}

	OK(w, map[string]int{"success": success, "failed": failed})
}

// ────────────────────────────── Probe ──────────────────────────────

// Probe connects to a BMC and reads FRU information without saving a config.
func (h *BMCHandler) Probe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Host == "" || req.Username == "" || req.Password == "" {
		Error(w, http.StatusBadRequest, "host, username and password are required")
		return
	}
	if req.Port == 0 {
		req.Port = 623
	}

	result, err := bmc.ProbeDevice(r.Context(), bmc.Config{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Redfish:  req.Protocol == "redfish",
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, result)
}

// Refresh probes an existing BMC config and updates its discovered fields
// (vendor, model, serial, mac, name).
func (h *BMCHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	cfg, err := h.store.GetBMCConfig(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "BMC config not found")
		return
	}

	result, err := bmc.ProbeDevice(r.Context(), bmc.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		Redfish:  cfg.Protocol == "redfish",
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update config with probe results (only overwrite non-empty values).
	if result.Vendor != "" {
		cfg.Vendor = result.Vendor
	}
	if result.Model != "" {
		cfg.Model = result.Model
	}
	if result.Serial != "" {
		cfg.Serial = result.Serial
	}
	if result.MAC != "" {
		cfg.MAC = result.MAC
	}
	if result.Name != "" {
		cfg.Name = result.Name
	}

	if err := h.store.UpdateBMCConfig(r.Context(), cfg); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, toBMCConfigResponse(*cfg))
}

// ────────────────────────────── Power actions ──────────────────────────────

// powerAction is a shared helper that resolves a BMC config from the URL {id}
// parameter, creates a controller, executes fn, and writes the result.
func (h *BMCHandler) powerAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, bmc.Controller) (any, error)) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	cfg, err := h.store.GetBMCConfig(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "BMC config not found")
		return
	}

	ctrl, err := bmc.NewController(bmc.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		Redfish:  cfg.Protocol == "redfish",
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer ctrl.Close()

	result, err := fn(r.Context(), ctrl)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, result)
}

func (h *BMCHandler) PowerOn(w http.ResponseWriter, r *http.Request) {
	h.powerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) (any, error) {
		if err := ctrl.PowerOn(ctx); err != nil {
			return nil, err
		}
		return map[string]string{"status": "on"}, nil
	})
}

func (h *BMCHandler) PowerOff(w http.ResponseWriter, r *http.Request) {
	h.powerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) (any, error) {
		if err := ctrl.PowerOff(ctx); err != nil {
			return nil, err
		}
		return map[string]string{"status": "off"}, nil
	})
}

func (h *BMCHandler) PowerRestart(w http.ResponseWriter, r *http.Request) {
	h.powerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) (any, error) {
		if err := ctrl.PowerRestart(ctx); err != nil {
			return nil, err
		}
		return map[string]string{"status": "restart"}, nil
	})
}

func (h *BMCHandler) PowerStatus(w http.ResponseWriter, r *http.Request) {
	h.powerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) (any, error) {
		state, err := ctrl.PowerStatus(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{"status": string(state)}, nil
	})
}

type bootDeviceRequest struct {
	Device string `json:"device"`
}

func (h *BMCHandler) SetBootDevice(w http.ResponseWriter, r *http.Request) {
	var req bootDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Device == "" {
		Error(w, http.StatusBadRequest, "device is required")
		return
	}

	h.powerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) (any, error) {
		if err := ctrl.SetBootDevice(ctx, bmc.BootDevice(req.Device)); err != nil {
			return nil, err
		}
		return map[string]string{"device": req.Device}, nil
	})
}

// ────────────────────────────── Batch operations ──────────────────────────────

type batchIDsRequest struct {
	IDs []int64 `json:"ids"`
}

type batchResultItem struct {
	ID      int64  `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Status  string `json:"status,omitempty"`
}

// batchPowerAction runs a power operation against each BMC config sequentially.
// One failure does not stop the others.
func (h *BMCHandler) batchPowerAction(w http.ResponseWriter, r *http.Request, actionFn func(context.Context, bmc.Controller) error) {
	var req batchIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results := make([]batchResultItem, 0, len(req.IDs))
	for _, id := range req.IDs {
		item := batchResultItem{ID: id}
		cfg, err := h.store.GetBMCConfig(r.Context(), id)
		if err != nil {
			item.Error = "config not found"
			results = append(results, item)
			continue
		}

		ctrl, err := bmc.NewController(bmc.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Username: cfg.Username,
			Password: cfg.Password,
			Redfish:  cfg.Protocol == "redfish",
		})
		if err != nil {
			item.Error = err.Error()
			results = append(results, item)
			continue
		}

		if err := actionFn(r.Context(), ctrl); err != nil {
			item.Error = err.Error()
		} else {
			item.Success = true
		}
		ctrl.Close()
		results = append(results, item)
	}

	OK(w, map[string]any{"results": results})
}

func (h *BMCHandler) BatchPowerOn(w http.ResponseWriter, r *http.Request) {
	h.batchPowerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) error {
		return ctrl.PowerOn(ctx)
	})
}

func (h *BMCHandler) BatchPowerOff(w http.ResponseWriter, r *http.Request) {
	h.batchPowerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) error {
		return ctrl.PowerOff(ctx)
	})
}

func (h *BMCHandler) BatchRestart(w http.ResponseWriter, r *http.Request) {
	h.batchPowerAction(w, r, func(ctx context.Context, ctrl bmc.Controller) error {
		return ctrl.PowerRestart(ctx)
	})
}

func (h *BMCHandler) BatchStatus(w http.ResponseWriter, r *http.Request) {
	var req batchIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results := make([]batchResultItem, 0, len(req.IDs))
	for _, id := range req.IDs {
		item := batchResultItem{ID: id}
		cfg, err := h.store.GetBMCConfig(r.Context(), id)
		if err != nil {
			item.Error = "config not found"
			results = append(results, item)
			continue
		}

		ctrl, err := bmc.NewController(bmc.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Username: cfg.Username,
			Password: cfg.Password,
			Redfish:  cfg.Protocol == "redfish",
		})
		if err != nil {
			item.Error = err.Error()
			results = append(results, item)
			continue
		}

		state, err := ctrl.PowerStatus(r.Context())
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Success = true
			item.Status = string(state)
		}
		ctrl.Close()
		results = append(results, item)
	}

	OK(w, map[string]any{"results": results})
}
