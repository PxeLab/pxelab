package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"context"
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
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

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// StoreHandler exposes the community store inside PxeLab.
type StoreHandler struct {
	store      store.Interface
	endpoint   string
	client     *http.Client
	cache      *storeCache
	catalogDir string           // local netboot catalog dir; netboot_distro imports are written here
	netbootMgr *netboot.Manager // reloaded after a netboot_distro import
}

type storeCache struct {
	mu       sync.RWMutex
	catalog  *StoreCatalog
	cachedAt time.Time
	ttl      time.Duration
}

// NewStoreHandler creates a handler that talks to hub.pxelab.com.
// catalogDir/netbootMgr may be nil — netboot_distro imports then only create a
// Profile without touching the local catalog.
func NewStoreHandler(st store.Interface, catalogDir string, netbootMgr *netboot.Manager) *StoreHandler {
	return &StoreHandler{
		store:      st,
		endpoint:   DefaultStoreEndpoint,
		client:     &http.Client{Timeout: 15 * time.Second},
		cache:      &storeCache{ttl: 5 * time.Minute},
		catalogDir: catalogDir,
		netbootMgr: netbootMgr,
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

	// Import scripts — reuse library scripts with identical name/type/content so
	// repeated imports stay idempotent and don't pollute the shared script
	// library (脚本库) with duplicates.
	library, _ := h.store.ListScripts(r.Context())
	for _, sc := range content.Scripts {
		scriptType := sc.Type
		if scriptType == "" {
			scriptType = "shell"
		}
		script := findLibraryScript(library, sc.Name, scriptType, sc.Content)
		if script == nil {
			created := &models.Script{
				Name:        sc.Name,
				Type:        scriptType,
				Content:     sc.Content,
				Description: sc.Description,
			}
			if err := h.store.CreateScript(r.Context(), created); err != nil {
				Error(w, http.StatusInternalServerError, "导入脚本失败: "+err.Error())
				h.store.DeleteBaseline(r.Context(), blID)
				return
			}
			library = append(library, *created)
			script = created
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

// importProfileConfig parameterizes importProfile, which turns a boot_template
// store item (an iPXE script) into a local Profile.
type importProfileConfig struct {
	defaultIDPrefix string
	parseErrMsg     string
	emptyErrMsg     string
	auditMsg        string
	responseType    string
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

// netbootDistroStoreContent is the "content" shape of netboot_distro store
// items. Newer hub items carry a full netboot.Distro descriptor; older items
// used "distro_name" instead of "name" — both are accepted.
type netbootDistroStoreContent struct {
	netboot.Distro
	DistroName string `json:"distro_name"` // legacy field from older hub items
}

// importHubNetbootDistro imports a hub netboot_distro item. The distro
// descriptor is written back into the local netboot catalog (store is the
// source of truth — a same-name YAML is overwritten), the catalog manager is
// reloaded, and a ready-to-assign Profile covering all enabled versions is
// created. Used by both online (ImportItem) and offline (ImportLocalItem)
// imports.
func (h *StoreHandler) importHubNetbootDistro(w http.ResponseWriter, r *http.Request, detail *StoreItemDetail) {
	var content netbootDistroStoreContent
	if err := json.Unmarshal(stripUTF8BOM(detail.Content), &content); err != nil {
		Error(w, http.StatusBadRequest, "解析网络启动内容失败: "+err.Error())
		return
	}
	distro := content.Distro
	if distro.Name == "" {
		distro.Name = content.DistroName
	}
	if distro.Name == "" {
		distro.Name = detail.Name
	}
	if distro.Name == "" {
		Error(w, http.StatusBadRequest, "网络启动内容缺少发行版名称")
		return
	}

	entries := make([]models.MenuEntry, 0, len(distro.Versions))
	for _, v := range distro.Versions {
		if !v.Enabled {
			continue
		}
		hasBootFile := (v.Remote != nil && v.Remote.Kernel != "") || (v.Local != nil && v.Local.Kernel != "")
		if !hasBootFile {
			continue
		}
		label := v.Name
		if !isPrintableASCII(label) {
			label = v.Codename
		}
		if label == "" {
			continue
		}
		entries = append(entries, menuEntryFromVersion(v, label))
	}
	if len(entries) == 0 {
		Error(w, http.StatusBadRequest, "该网络启动项没有可用版本")
		return
	}

	// Write back into the local catalog and reload, so the imported distro is
	// usable by the catalog page, profile creation and install-task answer
	// injection — not just the generated Profile.
	if h.catalogDir != "" && h.netbootMgr != nil {
		if err := h.saveDistroToCatalog(&distro); err != nil {
			Error(w, http.StatusInternalServerError, "写入本地目录失败: "+err.Error())
			return
		}
	}

	profileID := toSafeID(detail.ID)
	if profileID == "" || profileID == "imported" {
		profileID = "netboot-" + toSafeID(distro.Name)
	}
	if existing, _ := h.store.GetProfile(r.Context(), profileID); existing != nil {
		profileID = profileID + "-" + fmt.Sprintf("%d", time.Now().Unix())
	}
	profileName := detail.Name
	if !isPrintableASCII(profileName) {
		profileName = toSafeID(distro.Name)
	}
	// profiles.name has a UNIQUE constraint in the SQLite store — dedupe like
	// the ID above so repeated imports don't fail on the name collision.
	profileName = h.uniqueProfileName(r, profileName)
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
	RecordAudit(r.Context(), h.store, models.AuditCreate, "profile", profileID, remoteIP(r), "从商店导入网络启动发行版: "+distro.Name)
	Created(w, map[string]any{
		"id":          profileID,
		"name":        profileName,
		"type":        "netboot_distro",
		"store_item":  detail.ID,
		"distro_name": distro.Name,
	})
}

// saveDistroToCatalog persists the distro as a YAML file in the local catalog
// directory and reloads the manager. If an existing catalog file already
// defines a distro with the same name (e.g. an embedded seed file), that file
// is overwritten so imports update in place instead of creating duplicates.
// On reload failure the written file is rolled back so the on-disk catalog
// stays consistent with the in-memory one.
func (h *StoreHandler) saveDistroToCatalog(distro *netboot.Distro) error {
	path := filepath.Join(h.catalogDir, catalogFileName(h.catalogDir, distro.Name))
	// Remember prior content so a failed reload can be rolled back.
	oldContent, readErr := os.ReadFile(path)
	existed := readErr == nil
	if err := netboot.SaveDistro(path, distro); err != nil {
		return err
	}
	cat, err := netboot.LoadCatalog(h.catalogDir)
	if err != nil || len(cat.Distros) == 0 {
		if existed {
			os.WriteFile(path, oldContent, 0644)
		} else {
			os.Remove(path)
		}
		if err == nil {
			err = fmt.Errorf("catalog reload returned no distros")
		}
		return err
	}
	h.netbootMgr.Reload(cat)
	return nil
}

// catalogFileName returns the file name to use for a distro: the existing file
// that already defines it (matched by distro name), or a name-derived file
// name for new distros.
func catalogFileName(catalogDir, distroName string) string {
	fallback := toSafeID(distroName) + ".yaml"
	entries, err := os.ReadDir(catalogDir)
	if err != nil {
		return fallback
	}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		if d, err := netboot.LoadDistro(filepath.Join(catalogDir, e.Name())); err == nil && d.Name == distroName {
			return e.Name()
		}
	}
	return fallback
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
		h.importHubNetbootDistro(w, r, &detail)
	default:
		Error(w, http.StatusBadRequest, "不支持的导入类型: "+detail.Type)
	}
}

// importProfile creates a Profile from a boot_template store item whose
// content is a raw iPXE script string.
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

// findLibraryScript looks up a script in the shared script library that matches
// name/type/content exactly, so imports can reuse it instead of duplicating.
func findLibraryScript(library []models.Script, name, typ, content string) *models.Script {
	for i := range library {
		s := library[i]
		sType := s.Type
		if sType == "" {
			sType = "shell"
		}
		if s.Name == name && sType == typ && s.Content == content {
			return &s
		}
	}
	return nil
}

// uniqueProfileName appends a numeric suffix when a profile with the same
// name already exists (profiles.name is UNIQUE in the SQLite store).
func (h *StoreHandler) uniqueProfileName(r *http.Request, name string) string {
	profiles, err := h.store.ListProfiles(r.Context())
	if err != nil {
		return name
	}
	taken := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		taken[p.Name] = true
	}
	if !taken[name] {
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s (%d)", name, i)
		if !taken[candidate] {
			return candidate
		}
	}
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
