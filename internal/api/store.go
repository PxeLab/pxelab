package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"context"
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

const DefaultStoreEndpoint = "https://hub.pxelab.com"

// ---------------------------------------------------------------------------
// Types — API contracts with hub.pxelab.com
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

// StoreCatalog is the catalog response from hub.pxelab.com.
type StoreCatalog struct {
	Items []StoreItem `json:"items"`
	Total int         `json:"total"`
}

// StoreItemDetail is the full item returned from the detail endpoint.
type StoreItemDetail struct {
	ID                string            `json:"id"`
	Type              string            `json:"type"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	Version           string            `json:"version"`
	Author            string            `json:"author"`
	Tags              []string          `json:"tags,omitempty"`
	Content           json.RawMessage   `json:"content"`
	Source            string            `json:"source,omitempty"`
	Releases          []string          `json:"releases,omitempty"`
	CreatedAt         string            `json:"created_at,omitempty"`
	UpdatedAt         string            `json:"updated_at,omitempty"`
	TemplateVariables map[string]string `json:"template_variables,omitempty"`
	VariableValues    map[string]string `json:"variable_values,omitempty"`
}

// -- importable content shapes ----------------------------------------------

// baselineStoreContent is the "content" field of a baseline-type store item.
type baselineStoreContent struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	OSFilter    string                  `json:"os_filter,omitempty"`
	Variables   string                  `json:"variables,omitempty"`
	Scripts     []baselineScriptContent `json:"scripts"`
}

type baselineScriptContent struct {
	Seq         int    `json:"seq"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Description string `json:"description,omitempty"`
}

// bootTemplateStoreContent mirrors the fields from a boot_template store item
// that are needed to create a Profile with a custom iPXE script.
type bootTemplateStoreContent struct {
	TemplateVariables map[string]string `json:"template_variables,omitempty"`
	VariableValues    map[string]string `json:"variable_values,omitempty"`
	Releases          []string          `json:"releases,omitempty"`
	Source            string            `json:"source,omitempty"`
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

// NewStoreHandler creates a handler that talks to hub.pxelab.com.
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
	case "boot_template":
		h.importBootTemplate(w, r, detail)
	case "netboot_distro":
		h.importHubNetbootDistro(w, r, detail)
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
	if err := json.Unmarshal(stripUTF8BOM(body), &cat); err != nil {
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
	if err := json.Unmarshal(stripUTF8BOM(body), &detail); err != nil {
		return nil, fmt.Errorf("解析应用详情失败: %w", err)
	}
	return &detail, nil
}

// stripUTF8BOM removes a leading UTF-8 byte-order mark (EF BB BF).
// hub.pxelab.com serves its JSON payloads with a BOM prefix, which
// encoding/json rejects; tolerate it so catalog/detail parsing succeeds.
func stripUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
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
		script := &models.Script{
			Name:        sc.Name,
			Type:        scriptType,
			Content:     sc.Content,
			Description: sc.Description,
		}
		if err := h.store.CreateScript(r.Context(), script); err != nil {
			Error(w, http.StatusInternalServerError, "导入脚本失败: "+err.Error())
			h.store.DeleteBaseline(r.Context(), blID)
			return
		}
		assign := models.BaselineScriptAssignment{
			BaselineID: blID,
			ScriptID:   script.ID,
			Seq:        sc.Seq,
		}
		if err := h.store.SetBaselineScripts(r.Context(), blID, []models.BaselineScriptAssignment{assign}); err != nil {
			Error(w, http.StatusInternalServerError, "关联脚本失败: "+err.Error())
			h.store.DeleteBaseline(r.Context(), blID)
			return
		}
	}

	// Fire download increment (best-effort, non-blocking).
	h.trackDownload(r.Context(), detail.Type, detail.ID)

	RecordAudit(r.Context(), h.store, models.AuditCreate, "baseline", blID, remoteIP(r), "从商店导入基线: "+blName)
	Created(w, map[string]any{
		"id":         blID,
		"name":       blName,
		"type":       "baseline",
		"store_item": detail.ID,
	})
}

// importProfileConfig holds the parameters that differ between importing a
// boot_template vs a netboot_distro store item — both produce a local Profile.
type importProfileConfig struct {
	defaultIDPrefix string // e.g. "boot-template-" or "netboot-"
	parseErrMsg     string
	emptyErrMsg     string
	auditMsg        string
	responseType    string // e.g. "boot_template" or "netboot_distro"
}

func (h *StoreHandler) importBootTemplate(w http.ResponseWriter, r *http.Request, detail *StoreItemDetail) {
	h.importProfile(w, r, detail, importProfileConfig{
		defaultIDPrefix: "boot-template-",
		parseErrMsg:     "解析启动模板内容失败: ",
		emptyErrMsg:     "启动模板内容为空",
		auditMsg:        "从商店导入启动模板: ",
		responseType:    "boot_template",
	})
}

// hubNetbootDistroContent is the "content" shape of netboot_distro store items
// served by hub.pxelab.com. Unlike boot_template (whose content is an iPXE
// script string), netboot_distro content is a distro descriptor whose versions
// carry bootable kernel/initrd URLs — mirroring the local netboot catalog.
type hubNetbootDistroContent struct {
	DistroName string                    `json:"distro_name"`
	Versions   []hubNetbootDistroVersion `json:"versions"`
}

type hubNetbootDistroVersion struct {
	Codename string `json:"codename"`
	Name     string `json:"name"`
	Arch     string `json:"arch"`
	Remote   *struct {
		Kernel string `json:"kernel"`
		Initrd string `json:"initrd"`
	} `json:"remote"`
	Enabled *bool `json:"enabled"`
}

// importHubNetbootDistro imports a hub netboot_distro item as a Profile whose
// boot menu contains one direct-boot entry per enabled, kernel-bearing version.
func (h *StoreHandler) importHubNetbootDistro(w http.ResponseWriter, r *http.Request, detail *StoreItemDetail) {
	var content hubNetbootDistroContent
	if err := json.Unmarshal(stripUTF8BOM(detail.Content), &content); err != nil {
		Error(w, http.StatusBadRequest, "解析网络启动内容失败: "+err.Error())
		return
	}

	entries := make([]models.MenuEntry, 0, len(content.Versions))
	for _, v := range content.Versions {
		if v.Enabled != nil && !*v.Enabled {
			continue
		}
		if v.Remote == nil || v.Remote.Kernel == "" {
			continue
		}
		label := v.Name
		if !isPrintableASCII(label) {
			label = v.Codename
		}
		if label == "" {
			continue
		}
		entries = append(entries, models.MenuEntry{
			Label:  label,
			Type:   "direct",
			Kernel: strPtr(v.Remote.Kernel),
			Initrd: strPtr(v.Remote.Initrd),
		})
	}
	if len(entries) == 0 {
		Error(w, http.StatusBadRequest, "该网络启动项没有可用版本")
		return
	}

	profileID := toSafeID(detail.ID)
	if existing, _ := h.store.GetProfile(r.Context(), profileID); existing != nil {
		profileID = profileID + "-" + fmt.Sprintf("%d", time.Now().Unix())
	}
	profileName := detail.Name
	if !isPrintableASCII(profileName) {
		profileName = toSafeID(detail.Name)
	}
	profile := &models.Profile{
		ID:          profileID,
		Name:        profileName,
		Description: detail.Description,
		Arch:        "x86_64",
	}
	if err := profile.SetMenu(&models.BootMenu{Entries: entries}); err != nil {
		Error(w, http.StatusInternalServerError, "构建菜单失败: "+err.Error())
		return
	}
	if err := h.store.CreateProfile(r.Context(), profile); err != nil {
		Error(w, http.StatusInternalServerError, "创建引导配置失败: "+err.Error())
		return
	}

	h.trackDownload(r.Context(), detail.Type, detail.ID)
	RecordAudit(r.Context(), h.store, models.AuditCreate, "profile", profileID, remoteIP(r), "从商店导入网络启动发行版: "+profileName)
	Created(w, map[string]any{
		"id":         profileID,
		"name":       profileName,
		"type":       "netboot_distro",
		"store_item": detail.ID,
	})
}

// trackDownload sends a fire-and-forget POST to increment the download count on hub.pxelab.com.
func (h *StoreHandler) trackDownload(ctx context.Context, itemType, itemID string) {
	go func() {
		dCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		u := h.storeURL(fmt.Sprintf("/dlincrement/%s/%s", itemType, itemID))
		req, err := http.NewRequestWithContext(dCtx, http.MethodPost, u, nil)
		if err != nil {
			log.Printf("[store] trackDownload: %v", err)
			return
		}
		resp, err := h.client.Do(req)
		if err != nil {
			log.Printf("[store] trackDownload: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

// ImportLocalItem imports a store item from a local JSON payload (offline import).
func (h *StoreHandler) ImportLocalItem(w http.ResponseWriter, r *http.Request) {
	var detail StoreItemDetail
	if err := json.NewDecoder(r.Body).Decode(&detail); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体: "+err.Error())
		return
	}
	if detail.ID == "" || detail.Type == "" {
		Error(w, http.StatusBadRequest, "id 和 type 不能为空")
		return
	}

	switch detail.Type {
	case "baseline":
		h.importBaseline(w, r, &detail)
	case "boot_template":
		h.importBootTemplate(w, r, &detail)
	case "netboot_distro":
		h.importNetbootDistro(w, r, &detail)
	default:
		Error(w, http.StatusBadRequest, "不支持的导入类型: "+detail.Type)
	}
}

func (h *StoreHandler) importNetbootDistro(w http.ResponseWriter, r *http.Request, detail *StoreItemDetail) {
	h.importProfile(w, r, detail, importProfileConfig{
		defaultIDPrefix: "netboot-",
		parseErrMsg:     "解析网络启动内容失败: ",
		emptyErrMsg:     "网络启动内容为空",
		auditMsg:        "从本地文件导入网络启动配置: ",
		responseType:    "netboot_distro",
	})
}

// importProfile is the shared implementation for creating a Profile from either
// a boot_template or netboot_distro store item.
func (h *StoreHandler) importProfile(w http.ResponseWriter, r *http.Request, detail *StoreItemDetail, cfg importProfileConfig) {
	var script string
	if err := json.Unmarshal(detail.Content, &script); err != nil {
		Error(w, http.StatusBadRequest, cfg.parseErrMsg+err.Error())
		return
	}
	if script == "" {
		Error(w, http.StatusBadRequest, cfg.emptyErrMsg)
		return
	}

	profileID := toSafeID(detail.ID)
	if profileID == "" {
		profileID = cfg.defaultIDPrefix + fmt.Sprintf("%d", time.Now().Unix())
	}

	existing, _ := h.store.GetProfile(r.Context(), profileID)
	if existing != nil {
		profileID = profileID + "-" + fmt.Sprintf("%d", time.Now().Unix())
	}

	profileName := detail.Name
	if profileName == "" {
		profileName = detail.ID
	}
	// iPXE's BIOS console font is ASCII-only — fall back to a safe ID-derived
	// name when the catalog name contains non-ASCII characters (e.g. Chinese).
	if !isPrintableASCII(profileName) {
		profileName = toSafeID(detail.Name)
	}

	finalScript := script
	if len(detail.VariableValues) > 0 {
		var preamble strings.Builder
		for k, v := range detail.VariableValues {
			preamble.WriteString(fmt.Sprintf("set %s %s\n", k, v))
		}
		preamble.WriteString("\n")
		finalScript = preamble.String() + script
	}

	menuScript := finalScript
	menu := &models.BootMenu{
		Entries: []models.MenuEntry{
			{
				Label:  profileName,
				Type:   "custom",
				Script: &menuScript,
			},
		},
	}

	profile := &models.Profile{
		ID:          profileID,
		Name:        profileName,
		Description: detail.Description,
		Arch:        "x86_64",
	}
	if err := profile.SetMenu(menu); err != nil {
		Error(w, http.StatusInternalServerError, "构建菜单失败: "+err.Error())
		return
	}
	if len(detail.VariableValues) > 0 {
		if err := profile.SetVariablesMap(detail.VariableValues); err != nil {
			Error(w, http.StatusInternalServerError, "设置变量失败: "+err.Error())
			return
		}
	}

	if err := h.store.CreateProfile(r.Context(), profile); err != nil {
		Error(w, http.StatusInternalServerError, "创建引导配置失败: "+err.Error())
		return
	}

	h.trackDownload(r.Context(), detail.Type, detail.ID)

	RecordAudit(r.Context(), h.store, models.AuditCreate, "profile", profileID, remoteIP(r), cfg.auditMsg+profileName)
	Created(w, map[string]any{
		"id":         profileID,
		"name":       profileName,
		"type":       cfg.responseType,
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
