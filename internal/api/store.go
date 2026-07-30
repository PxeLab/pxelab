package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

const DefaultStoreEndpoint = "https://store.pxelab.com"

// ---------------------------------------------------------------------------
// Types — API contracts with store.pxelab.com
// ---------------------------------------------------------------------------

// StoreItem is a lightweight item in the catalog listing.
type StoreItem struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// StoreCatalog is the catalog response from store.pxelab.com.
type StoreCatalog struct {
	Items []StoreItem `json:"items"`
	Total int         `json:"total"`
}

// StoreItemDetail is the full item returned from the detail endpoint.
type StoreItemDetail struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Version     string          `json:"version"`
	Author      string          `json:"author"`
	Tags        []string        `json:"tags,omitempty"`
	Content     json.RawMessage `json:"content"`
}

// -- importable content shapes ----------------------------------------------

// baselineStoreContent is the "content" field of a baseline-type store item.
type baselineStoreContent struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	OSFilter    string                 `json:"os_filter,omitempty"`
	Variables   string                 `json:"variables,omitempty"`
	Scripts     []baselineScriptContent `json:"scripts"`
}

type baselineScriptContent struct {
	Seq         int    `json:"seq"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Description string `json:"description,omitempty"`
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// StoreHandler exposes the community store inside PxeLab.
type StoreHandler struct {
	store    store.Interface
	endpoint string
	client   *http.Client
	cache    *storeCache
}

type storeCache struct {
	mu       sync.RWMutex
	catalog  *StoreCatalog
	cachedAt time.Time
	ttl      time.Duration
}

// NewStoreHandler creates a handler that talks to store.pxelab.com.
func NewStoreHandler(st store.Interface) *StoreHandler {
	return &StoreHandler{
		store:    st,
		endpoint: DefaultStoreEndpoint,
		client:   &http.Client{Timeout: 15 * time.Second},
		cache:    &storeCache{ttl: 5 * time.Minute},
	}
}

// WithEndpoint overrides the store endpoint (used in tests).
func (h *StoreHandler) WithEndpoint(ep string) *StoreHandler {
	h.endpoint = strings.TrimRight(ep, "/")
	return h
}

func (h *StoreHandler) storeURL(path string) string {
	return h.endpoint + "/api/v1" + path
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// ListCatalog returns the store catalog (uses in-memory cache).
func (h *StoreHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	h.cache.mu.RLock()
	if h.cache.catalog != nil && time.Since(h.cache.cachedAt) < h.cache.ttl {
		cat := h.cache.catalog
		h.cache.mu.RUnlock()
		OK(w, map[string]any{"catalog": cat})
		return
	}
	h.cache.mu.RUnlock()

	cat, err := h.fetchCatalog()
	if err != nil {
		Error(w, http.StatusBadGateway, "无法连接应用商店: "+err.Error())
		return
	}

	h.cache.mu.Lock()
	h.cache.catalog = cat
	h.cache.cachedAt = time.Now()
	h.cache.mu.Unlock()

	OK(w, map[string]any{"catalog": cat})
}

// GetItem returns a full store item detail (no caching — always live).
func (h *StoreHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	itemType := chi.URLParam(r, "type")
	itemID := chi.URLParam(r, "id")

	detail, err := h.fetchItem(itemType, itemID)
	if err != nil {
		Error(w, http.StatusBadGateway, "获取应用详情失败: "+err.Error())
		return
	}
	OK(w, detail)
}

// ImportItem fetches an item from the store and saves it into the local database.
func (h *StoreHandler) ImportItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ItemID   string `json:"item_id"`
		ItemType string `json:"item_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.ItemID == "" || req.ItemType == "" {
		Error(w, http.StatusBadRequest, "item_id 和 item_type 不能为空")
		return
	}

	detail, err := h.fetchItem(req.ItemType, req.ItemID)
	if err != nil {
		Error(w, http.StatusBadGateway, "获取应用详情失败: "+err.Error())
		return
	}

	switch detail.Type {
	case "baseline":
		h.importBaseline(w, r, detail)
	default:
		Error(w, http.StatusBadRequest, "不支持的导入类型: "+detail.Type)
	}
}

// ---------------------------------------------------------------------------
// Store API client
// ---------------------------------------------------------------------------

func (h *StoreHandler) fetchCatalog() (*StoreCatalog, error) {
	resp, err := h.client.Get(h.storeURL("/catalog"))
	if err != nil {
		return nil, fmt.Errorf("请求商店目录失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("商店返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var cat StoreCatalog
	if err := json.Unmarshal(body, &cat); err != nil {
		return nil, fmt.Errorf("解析商店目录失败: %w", err)
	}
	return &cat, nil
}

func (h *StoreHandler) fetchItem(itemType, itemID string) (*StoreItemDetail, error) {
	url := h.storeURL(fmt.Sprintf("/items/%s/%s", itemType, itemID))
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求应用详情失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("商店返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var detail StoreItemDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("解析应用详情失败: %w", err)
	}
	return &detail, nil
}

// ---------------------------------------------------------------------------
// Importers
// ---------------------------------------------------------------------------

func (h *StoreHandler) importBaseline(w http.ResponseWriter, r *http.Request, detail *StoreItemDetail) {
	var content baselineStoreContent
	if err := json.Unmarshal(detail.Content, &content); err != nil {
		Error(w, http.StatusBadRequest, "解析基线内容失败: "+err.Error())
		return
	}

	blName := content.Name
	if blName == "" {
		blName = detail.Name
	}

	// Build a short ID from the store item ID or name.
	blID := detail.ID
	if blID == "" {
		blID = toSafeID(blName)
	}

	// Collision: append a suffix if the ID already exists.
	existing, _ := h.store.GetBaseline(r.Context(), blID)
	if existing != nil {
		blID = blID + "-" + fmt.Sprintf("%d", time.Now().Unix())
	}

	bl := &models.Baseline{
		ID:          blID,
		Name:        blName,
		Description: content.Description,
		OSFilter:    content.OSFilter,
	}
	if content.Variables != "" {
		bl.Variables = content.Variables
	}

	if err := h.store.CreateBaseline(r.Context(), bl); err != nil {
		Error(w, http.StatusInternalServerError, "创建基线失败: "+err.Error())
		return
	}

	// Import scripts.
	for _, sc := range content.Scripts {
		scriptType := sc.Type
		if scriptType == "" {
			scriptType = "shell"
		}
		script := &models.BaselineScript{
			BaselineID:  blID,
			Seq:         sc.Seq,
			Name:        sc.Name,
			Type:        scriptType,
			Content:     sc.Content,
			Description: sc.Description,
		}
		if err := h.store.UpsertBaselineScript(r.Context(), script); err != nil {
			// Log but don't fail — partial import is better than nothing.
			Error(w, http.StatusInternalServerError, "导入脚本失败: "+err.Error())
			// Clean up the baseline we just created.
			h.store.DeleteBaseline(r.Context(), blID)
			return
		}
	}

	RecordAudit(r.Context(), h.store, models.AuditCreate, "baseline", blID, remoteIP(r), "从商店导入基线: "+blName)
	Created(w, map[string]any{
		"id":         blID,
		"name":       blName,
		"type":       "baseline",
		"store_item": detail.ID,
	})
}

// toSafeID converts an arbitrary string into a safe baseline ID.
func toSafeID(s string) string {
	safe := strings.Builder{}
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			safe.WriteRune(r)
		} else if r == ' ' || r == '.' {
			safe.WriteRune('-')
		}
	}
	id := safe.String()
	id = strings.Trim(id, "-_")
	if id == "" {
		id = "imported"
	}
	return id
}
