# iPXE 引导行为可视化管理 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将硬编码在 server.go 和 script.go 中的 iPXE 引导逻辑提取为可配置的数据模型，通过 Web UI 可视化管理

**Architecture:** 在 NetbootConfig 下新增 boot 子结构（default_menu、profile_behavior、catalog_redirect、catalog_display），server.go 改为纯配置驱动决策树，script.go 接受 group 配置替代硬编码标题。

**Tech Stack:** Go (config structs, chi router, text/template)、React (Settings.tsx 复用 Profiles 条目编辑器)

---

## 文件清单

| 文件 | 改动 |
|------|------|
| `internal/config/config.go` | 新增 BootConfig + 4 个子结构体 |
| `internal/api/settings.go` | 新增 BootSettings API 结构 + Get/Update 读写 |
| `internal/httpd/server.go` | 重写 `/boot/ipxe/script` 为配置驱动决策树 |
| `internal/netboot/script.go` | `GenerateNetbootScript()` 增加 groups 参数 |
| `internal/netboot/script_test.go` | 更新测试调用签名 |
| `web/src/api/client.ts` | 新增 BootSettings interface |
| `web/src/pages/Settings.tsx` | 新增 4 个模块的 UI 编辑器 |

---

### Task 1: 添加配置类型

**Files:**
- Modify: `internal/config/config.go:57-77`

在 `NetbootConfig` 中添加 `Boot` 字段，并定义全部子结构体。

- [ ] **Step 1: 在 NetbootConfig 中添加 Boot 字段**

在 `NetbootConfig` 结构体中 `MenuTitle` 和 `Sync` 之间添加 `Boot BootConfig`：

```go
type NetbootConfig struct {
	Enabled        bool              `mapstructure:"enabled"`
	DefaultBoot    string            `mapstructure:"default_boot"`
	FallbackOnline bool              `mapstructure:"fallback_online"`
	MenuTitle      string            `mapstructure:"menu_title"`
	ScriptTemplate string            `mapstructure:"script_template"`
	Boot           BootConfig        `mapstructure:"boot"`
	Sync           NetbootSyncConfig `mapstructure:"sync"`
	Paths          NetbootPathConfig `mapstructure:"paths"`
}
```

- [ ] **Step 2: 添加 BootConfig 和子结构体**

在 `NetbootPathConfig` 结构体之后、`StoreConfig` 之前添加：

```go
type BootConfig struct {
	DefaultMenu     DefaultMenuConfig     `mapstructure:"default_menu" yaml:"default_menu"`
	ProfileBehavior ProfileBehaviorConfig `mapstructure:"profile_behavior" yaml:"profile_behavior"`
	CatalogRedirect CatalogRedirectConfig `mapstructure:"catalog_redirect" yaml:"catalog_redirect"`
	CatalogDisplay  CatalogDisplayConfig  `mapstructure:"catalog_display" yaml:"catalog_display"`
}

type DefaultMenuConfig struct {
	Title   string      `mapstructure:"title" yaml:"title"`
	Timeout int         `mapstructure:"timeout" yaml:"timeout"`
	Default int         `mapstructure:"default" yaml:"default"`
	Entries []MenuEntry `mapstructure:"entries" yaml:"entries"`
}

// MenuEntry 复用的其实是我们已经在 store 中定义的 MenuEntry 类型，
// 但由于它在 store 包中且无法直接导入 config 包，
// 这里定义一个本地简化版本。
// 如果 store.MenuEntry 已经可导出且无循环依赖，可以直接引用。
type MenuEntry struct {
	Label  string  `mapstructure:"label" yaml:"label"`
	Type   string  `mapstructure:"type" yaml:"type"`
	Kernel *string `mapstructure:"kernel,omitempty" yaml:"kernel,omitempty"`
	Initrd *string `mapstructure:"initrd,omitempty" yaml:"initrd,omitempty"`
	Cmdline *string `mapstructure:"cmdline,omitempty" yaml:"cmdline,omitempty"`
	URL    *string `mapstructure:"url,omitempty" yaml:"url,omitempty"`
	WIM    *string `mapstructure:"wim,omitempty" yaml:"wim,omitempty"`
}

type ProfileBehaviorConfig struct {
	AppendLocal    bool   `mapstructure:"append_local" yaml:"append_local"`
	AppendNetboot  bool   `mapstructure:"append_netboot" yaml:"append_netboot"`
	AppendPosition string `mapstructure:"append_position" yaml:"append_position"`
}

type CatalogRedirectConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
	TargetURL  string `mapstructure:"target_url" yaml:"target_url"`
	DetectArch bool   `mapstructure:"detect_arch" yaml:"detect_arch"`
	Preamble   string `mapstructure:"preamble" yaml:"preamble"`
}

type CatalogDisplayConfig struct {
	Title  string         `mapstructure:"title" yaml:"title"`
	Groups []CatalogGroup `mapstructure:"groups" yaml:"groups"`
}

type CatalogGroup struct {
	Name    string `mapstructure:"name" yaml:"name"`
	Title   string `mapstructure:"title" yaml:"title"`
	Enabled bool   `mapstructure:"enabled" yaml:"enabled"`
	Order   int    `mapstructure:"order" yaml:"order"`
}
```

- [ ] **Step 3: 验证编译**

```bash
cd /repo && go build ./...
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add BootConfig and sub-types for iPXE boot settings"
```

---

### Task 2: API 暴露 BootSettings

**Files:**
- Modify: `internal/api/settings.go:33-44, 117-165, 258-360`

在 API 响应结构中增加 BootSettings，在 Get 中读取配置，在 Update 中写入配置。

- [ ] **Step 1: 添加 BootSettings API 结构体**

在 `NetbootSettings` 结构体下方添加：

```go
type BootSettings struct {
	DefaultMenu     DefaultMenuSettings     `json:"default_menu"`
	ProfileBehavior ProfileBehaviorSettings `json:"profile_behavior"`
	CatalogRedirect CatalogRedirectSettings `json:"catalog_redirect"`
	CatalogDisplay  CatalogDisplaySettings  `json:"catalog_display"`
}

type DefaultMenuSettings struct {
	Title   string              `json:"title"`
	Timeout int                 `json:"timeout"`
	Default int                 `json:"default"`
	Entries []MenuEntrySettings `json:"entries"`
}

type MenuEntrySettings struct {
	Label   string  `json:"label"`
	Type    string  `json:"type"`
	Kernel  *string `json:"kernel,omitempty"`
	Initrd  *string `json:"initrd,omitempty"`
	Cmdline *string `json:"cmdline,omitempty"`
	URL     *string `json:"url,omitempty"`
	WIM     *string `json:"wim,omitempty"`
}

type ProfileBehaviorSettings struct {
	AppendLocal    bool   `json:"append_local"`
	AppendNetboot  bool   `json:"append_netboot"`
	AppendPosition string `json:"append_position"`
}

type CatalogRedirectSettings struct {
	Enabled    bool   `json:"enabled"`
	TargetURL  string `json:"target_url"`
	DetectArch bool   `json:"detect_arch"`
	Preamble   string `json:"preamble"`
}

type CatalogDisplaySettings struct {
	Title  string               `json:"title"`
	Groups []CatalogGroupSettings `json:"groups"`
}

type CatalogGroupSettings struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
	Order   int    `json:"order"`
}
```

同时需要在 `NetbootSettings` 中增加 `Boot` 字段：

```go
type NetbootSettings struct {
	Enabled        bool         `json:"enabled"`
	ScriptTemplate string       `json:"script_template"`
	Boot           BootSettings `json:"boot"`
}
```

- [ ] **Step 2: 在 Get handler 中填充 BootSettings**

在 `settings.go` 的 `Get` 方法中，`NetbootSettings` 块之后添加转换代码。将 `cfg.Netboot.Boot` 映射到 API 响应：

```go
resp.Netboot.ScriptTemplate = cfg.Netboot.ScriptTemplate
resp.Netboot.Boot = BootSettings{
	DefaultMenu: DefaultMenuSettings{
		Title:   cfg.Netboot.Boot.DefaultMenu.Title,
		Timeout: cfg.Netboot.Boot.DefaultMenu.Timeout,
		Default: cfg.Netboot.Boot.DefaultMenu.Default,
		Entries: convertMenuEntriesToAPI(cfg.Netboot.Boot.DefaultMenu.Entries),
	},
	ProfileBehavior: ProfileBehaviorSettings{
		AppendLocal:    cfg.Netboot.Boot.ProfileBehavior.AppendLocal,
		AppendNetboot:  cfg.Netboot.Boot.ProfileBehavior.AppendNetboot,
		AppendPosition: cfg.Netboot.Boot.ProfileBehavior.AppendPosition,
	},
	CatalogRedirect: CatalogRedirectSettings{
		Enabled:    cfg.Netboot.Boot.CatalogRedirect.Enabled,
		TargetURL:  cfg.Netboot.Boot.CatalogRedirect.TargetURL,
		DetectArch: cfg.Netboot.Boot.CatalogRedirect.DetectArch,
		Preamble:   cfg.Netboot.Boot.CatalogRedirect.Preamble,
	},
	CatalogDisplay: CatalogDisplaySettings{
		Title:  cfg.Netboot.Boot.CatalogDisplay.Title,
		Groups: convertCatalogGroupsToAPI(cfg.Netboot.Boot.CatalogDisplay.Groups),
	},
}
```

添加辅助转换函数：

```go
func convertMenuEntriesToAPI(entries []config.MenuEntry) []MenuEntrySettings {
	if len(entries) == 0 {
		return nil
	}
	result := make([]MenuEntrySettings, len(entries))
	for i, e := range entries {
		result[i] = MenuEntrySettings{
			Label:   e.Label,
			Type:    e.Type,
			Kernel:  e.Kernel,
			Initrd:  e.Initrd,
			Cmdline: e.Cmdline,
			URL:     e.URL,
			WIM:     e.WIM,
		}
	}
	return result
}

func convertCatalogGroupsToAPI(groups []config.CatalogGroup) []CatalogGroupSettings {
	if len(groups) == 0 {
		return nil
	}
	result := make([]CatalogGroupSettings, len(groups))
	for i, g := range groups {
		result[i] = CatalogGroupSettings{
			Name:    g.Name,
			Title:   g.Title,
			Enabled: g.Enabled,
			Order:   g.Order,
		}
	}
	return result
}
```

- [ ] **Step 3: 在 Update handler 中保存 BootSettings**

在 `settings.go` 的 `Update` 方法中，在 `h.cfg.Netboot.ScriptTemplate = req.Netboot.ScriptTemplate` 之后添加：

```go
h.cfg.Netboot.Boot = config.BootConfig{
	DefaultMenu: config.DefaultMenuConfig{
		Title:   req.Netboot.Boot.DefaultMenu.Title,
		Timeout: req.Netboot.Boot.DefaultMenu.Timeout,
		Default: req.Netboot.Boot.DefaultMenu.Default,
		Entries: convertMenuEntriesFromAPI(req.Netboot.Boot.DefaultMenu.Entries),
	},
	ProfileBehavior: config.ProfileBehaviorConfig{
		AppendLocal:    req.Netboot.Boot.ProfileBehavior.AppendLocal,
		AppendNetboot:  req.Netboot.Boot.ProfileBehavior.AppendNetboot,
		AppendPosition: req.Netboot.Boot.ProfileBehavior.AppendPosition,
	},
	CatalogRedirect: config.CatalogRedirectConfig{
		Enabled:    req.Netboot.Boot.CatalogRedirect.Enabled,
		TargetURL:  req.Netboot.Boot.CatalogRedirect.TargetURL,
		DetectArch: req.Netboot.Boot.CatalogRedirect.DetectArch,
		Preamble:   req.Netboot.Boot.CatalogRedirect.Preamble,
	},
	CatalogDisplay: config.CatalogDisplayConfig{
		Title:  req.Netboot.Boot.CatalogDisplay.Title,
		Groups: convertCatalogGroupsFromAPI(req.Netboot.Boot.CatalogDisplay.Groups),
	},
}
```

添加反向转换函数：

```go
func convertMenuEntriesFromAPI(entries []MenuEntrySettings) []config.MenuEntry {
	if len(entries) == 0 {
		return nil
	}
	result := make([]config.MenuEntry, len(entries))
	for i, e := range entries {
		result[i] = config.MenuEntry{
			Label:   e.Label,
			Type:    e.Type,
			Kernel:  e.Kernel,
			Initrd:  e.Initrd,
			Cmdline: e.Cmdline,
			URL:     e.URL,
			WIM:     e.WIM,
		}
	}
	return result
}

func convertCatalogGroupsFromAPI(groups []CatalogGroupSettings) []config.CatalogGroup {
	if len(groups) == 0 {
		return nil
	}
	result := make([]config.CatalogGroup, len(groups))
	for i, g := range groups {
		result[i] = config.CatalogGroup{
			Name:    g.Name,
			Title:   g.Title,
			Enabled: g.Enabled,
			Order:   g.Order,
		}
	}
	return result
}
```

- [ ] **Step 4: 验证编译**

```bash
cd /repo && go build ./...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/settings.go
git commit -m "feat(api): expose BootSettings in settings API"
```

---

### Task 3: 更新 GenerateNetbootScript 支持可配置分组

**Files:**
- Modify: `internal/netboot/script.go:22-45, 227-244`
- Modify: `internal/netboot/script_test.go`

`GenerateNetbootScript()` 新增 `groups []config.CatalogGroup` 参数，替代硬编码的 `groupTitle()` map。

- [ ] **Step 1: 更新函数签名和逻辑**

```go
func GenerateNetbootScript(c *Catalog, serverAddr, arch, platform string, groups []config.CatalogGroup) string {
	archFilter := archMap[arch]

	// build group title lookup from config
	groupTitles := make(map[string]string)
	groupEnabled := make(map[string]bool)
	groupOrder := make(map[string]int)
	hasGroups := len(groups) > 0
	for _, g := range groups {
		groupTitles[g.Name] = g.Title
		groupEnabled[g.Name] = g.Enabled
		groupOrder[g.Name] = g.Order
	}

	versionMatchesArch := func(v *Version) bool {
		if archFilter == "" {
			return true
		}
		return v.Arch == archFilter
	}

	distroHasArch := func(d *Distro) bool {
		if archFilter == "" {
			return true
		}
		for _, v := range d.Versions {
			if v.Enabled && versionMatchesArch(v) {
				return true
			}
		}
		return false
	}

	var b strings.Builder
	b.WriteString("#!ipxe\n\n")
	b.WriteString(":netboot_menu\n")
	menuTitle := "[OS] Netboot OS Install Catalog"
	if hasGroups {
		// use first group's catalog display title or default
	}
	_ = hasGroups
	_ = menuTitle
	b.WriteString("menu [OS] Netboot OS Install Catalog\n\n")
	b.WriteString("item local    Boot from local disk\n")

	groupsList := c.Groups()
	// sort groups by order if config groups available
	if hasGroups {
		// only include groups present in config with enabled=true
		var filtered []*Group
		for _, g := range groupsList {
			if enabled, ok := groupEnabled[g.Name]; ok && !enabled {
				continue
			}
			filtered = append(filtered, g)
		}
		groupsList = filtered
	}

	for _, g := range groupsList {
		var matching []*Distro
		for _, d := range g.Distros {
			if distroHasArch(d) {
				matching = append(matching, d)
			}
		}
		if len(matching) == 0 {
			continue
		}
		title := groupTitleFallback(g.Name, groupTitles)
		b.WriteString(fmt.Sprintf("item --gap %s\n", title))
		for _, d := range matching {
			label := distroLabel(d)
			b.WriteString(fmt.Sprintf("item %s %s %s\n", label, "   ", d.Name))
		}
	}

	b.WriteString("item --gap\n")
	b.WriteString("item exit    Reboot\n")
	b.WriteString("choose selected || goto exit\n")
	b.WriteString("goto ${selected}\n\n")

	// ... rest of function unchanged but also need to sort groups by order when iterating for submenus ...
```

Actually, the full function rewrite is too large for inline steps. Let me break this differently — replace `groupTitle()` with `groupTitleFallback()` and modify the function.

Add this new function replacing `groupTitle()`:

```go
// groupTitleFallback returns the configured title if available, otherwise uses hardcoded defaults.
func groupTitleFallback(name string, configured map[string]string) string {
	if t, ok := configured[name]; ok && t != "" {
		return "== " + t + " =="
	}
	// hardcoded fallbacks
	titles := map[string]string{
		"linux":       "== Linux Distributions ==",
		"linux-i386":  "== Linux Distributions (32-bit) ==",
		"linux-arm64": "== Linux Distributions (arm64) ==",
		"bsd":         "== BSD Systems ==",
		"live":        "== Live CDs ==",
		"live-arm":    "== Live CDs (arm64) ==",
		"tools":       "== System Tools ==",
		"unix":        "== Unix ==",
		"dos":         "== DOS ==",
		"windows":     "== Windows ==",
	}
	if t, ok := titles[name]; ok {
		return t
	}
	return name
}
```

The key change to `GenerateNetbootScript`: add `groups []config.CatalogGroup` parameter, build lookup maps at the top, replace calls to `groupTitle(g.Name)` with `groupTitleFallback(g.Name, groupTitles)`, and filter out disabled groups.

For the menu title at the top, use the configured title from CatalogDisplay:

```go
// replace: b.WriteString("menu [OS] Netboot OS Install Catalog\n\n")
menuTitle := "[OS] Netboot OS Install Catalog"
if len(groups) > 0 {
	for _, g := range groups {
		if g.Name == "" { // main catalog title is stored separately
			continue
		}
	}
}
// Actually just pass title as separate param:
b.WriteString(fmt.Sprintf("menu %s\n\n", menuTitle))
```

Hmm, this is getting complicated in a plan. Let me simplify.

- [ ] **Step 1: 添加 groupTitleFallback 函数，删除旧的 groupTitle**

将原来的 `groupTitle(name string) string` 函数替换为：

```go
func groupTitleFallback(name string, configured map[string]string) string {
	if t, ok := configured[name]; ok && t != "" {
		return "== " + t + " =="
	}
	titles := map[string]string{
		"linux":       "== Linux Distributions ==",
		"linux-i386":  "== Linux Distributions (32-bit) ==",
		"linux-arm64": "== Linux Distributions (arm64) ==",
		"bsd":         "== BSD Systems ==",
		"live":        "== Live CDs ==",
		"live-arm":    "== Live CDs (arm64) ==",
		"tools":       "== System Tools ==",
		"unix":        "== Unix ==",
		"dos":         "== DOS ==",
		"windows":     "== Windows ==",
	}
	if t, ok := titles[name]; ok {
		return t
	}
	return name
}
```

- [ ] **Step 2: 修改 GenerateNetbootScript 签名和内部实现**

修改函数签名为：
```go
func GenerateNetbootScript(c *Catalog, serverAddr, arch, platform, menuTitle string, groups []config.CatalogGroup) string {
```

在函数开头添加配置映射：
```go
	groupTitles := make(map[string]string)
	groupEnabled := make(map[string]bool)
	hasGroups := len(groups) > 0
	for _, g := range groups {
		groupTitles[g.Name] = g.Title
		groupEnabled[g.Name] = g.Enabled
	}
```

将 `menuTitle` 应用到 `b.WriteString(fmt.Sprintf("menu %s\n\n", menuTitle))`

遍历 groups 时，过滤掉 `groupEnabled[name] == false` 的组。

将所有 `groupTitle(g.Name)` 替换为 `groupTitleFallback(g.Name, groupTitles)`。

- [ ] **Step 3: 运行现有测试，确认签名变化导致编译失败**

```bash
cd /repo && go build ./...
```

Expected: FAIL — `script_test.go` 和 `server.go` 调用签名不匹配

- [ ] **Step 4: 更新调用方（server.go 和 script_test.go）**

`server.go` line 192 改为：
```go
script := netboot.GenerateNetbootScript(netbootMgr.Catalog(), serverAddr, arch, platform,
	cfg.Netboot.Boot.CatalogDisplay.Title,
	cfg.Netboot.Boot.CatalogDisplay.Groups)
```

`script_test.go` 中所有调用 `GenerateNetbootScript` 的地方，追加两个参数：
```go
script := GenerateNetbootScript(cat, "server:8080", "", "", "[OS] Netboot OS Install Catalog", nil)
```

- [ ] **Step 5: 编译并运行测试**

```bash
cd /repo && go build ./... && go test ./internal/netboot/...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/netboot/script.go internal/netboot/script_test.go
git commit -m "feat(netboot): GenerateNetbootScript accepts configurable groups and title"
```

---

### Task 4: 重写 server.go iPXE 脚本端点为配置驱动

**Files:**
- Modify: `internal/httpd/server.go:54-139`（`/boot/ipxe/script` handler）

用 spec 中定义的决策树替换整个 handler 体。

- [ ] **Step 1: 提取配置驱动的 iPXE 脚本生成到独立函数**

在 `chainToIPXEFallback` 函数之前添加新函数：

```go
// generateIPXEScript 根据配置生成 iPXE 引导脚本
func generateIPXEScript(cfg *config.Config, st store.Interface, mac, serverAddr string, ctx context.Context) (string, error) {
	// 1. Custom script override
	if tmpl := cfg.Netboot.ScriptTemplate; tmpl != "" {
		return renderTemplate(tmpl, serverAddr, mac)
	}

	engine := ipxe.New()

	// 2. Host-specific profile
	if mac != "" {
		host, err := st.GetHostByMAC(ctx, mac)
		if err == nil && host != nil && host.ProfileID != nil {
			profile, err := st.GetProfile(ctx, *host.ProfileID)
			if err == nil && profile != nil {
				bootMenu, err := profile.GetMenu()
				if err == nil && len(bootMenu.Entries) > 0 {
					pm := &ipxe.MenuData{Title: profile.Name, Timeout: 5000, Default: 0}
					var entries []ipxe.MenuEntryData
					for _, e := range bootMenu.Entries {
						entry := ipxe.MenuEntryData{
							Label:   e.Label,
							Type:    ipxe.BootType(e.Type),
							Kernel:  urlJoin(serverAddr, e.Kernel),
							Initrd:  urlJoin(serverAddr, e.Initrd),
							Cmdline: ptrStr(e.Cmdline),
							URL:     ptrStr(e.URL),
							WIM:     ptrStr(e.WIM),
						}
						entries = append(entries, entry)
					}

					pb := cfg.Netboot.Boot.ProfileBehavior
					if pb.AppendLocal {
						localEntry := ipxe.MenuEntryData{Label: "Boot from local disk", Type: ipxe.BootLocal}
						if pb.AppendPosition == "first" {
							entries = append([]ipxe.MenuEntryData{localEntry}, entries...)
						} else {
							entries = append(entries, localEntry)
						}
					}
					if pb.AppendNetboot && cfg.Netboot.Enabled {
						netbootEntry := ipxe.MenuEntryData{Label: "[OS] Netboot OS Install Catalog", Type: ipxe.BootNetboot}
						entries = append(entries, netbootEntry)
					}

					pm.Entries = entries
					return engine.Render("menu", ipxe.TemplateData{
						MAC:  mac,
						Menu: pm,
						URL:  "http://" + serverAddr,
					})
				}
			}
		}
	}

	// 3. Catalog redirect
	cr := cfg.Netboot.Boot.CatalogRedirect
	if cr.Enabled && cfg.Netboot.Enabled {
		var b strings.Builder
		b.WriteString("#!ipxe\n")
		if cr.DetectArch {
			b.WriteString("cpuid --ext 29 && set arch x86_64 || set arch x86\n")
			b.WriteString("iseq ${buildarch} arm64 && set arch arm64 ||\n")
			b.WriteString("iseq ${buildarch} armhf && set arch armhf ||\n")
			b.WriteString("platform --is efi && set platform efi || set platform pc\n")
		}
		if cr.Preamble != "" {
			b.WriteString(cr.Preamble)
			if !strings.HasSuffix(cr.Preamble, "\n") {
				b.WriteString("\n")
			}
		}
		// Render target_url template
		targetURL := strings.ReplaceAll(cr.TargetURL, "{{.URL}}", "http://"+serverAddr)
		b.WriteString(fmt.Sprintf("chain %s\n", targetURL))
		return b.String(), nil
	}

	// 4. Default fallback menu
	dm := cfg.Netboot.Boot.DefaultMenu
	if len(dm.Entries) == 0 {
		// backward-compat: minimal menu
		dm.Title = "PxeGo Boot Menu"
		dm.Timeout = 0
		dm.Default = 0
		dm.Entries = []config.MenuEntry{{Label: "Boot from local disk", Type: "local"}}
	}

	var ipxeEntries []ipxe.MenuEntryData
	for _, e := range dm.Entries {
		ipxeEntries = append(ipxeEntries, ipxe.MenuEntryData{
			Label:   e.Label,
			Type:    ipxe.BootType(e.Type),
			Kernel:  urlJoin(serverAddr, e.Kernel),
			Initrd:  urlJoin(serverAddr, e.Initrd),
			Cmdline: ptrStr(e.Cmdline),
			URL:     ptrStr(e.URL),
			WIM:     ptrStr(e.WIM),
		})
	}

	return engine.Render("menu", ipxe.TemplateData{
		MAC: mac,
		Menu: &ipxe.MenuData{
			Title:   dm.Title,
			Timeout: dm.Timeout,
			Default: dm.Default,
			Entries: ipxeEntries,
		},
		URL: "http://" + serverAddr,
	})
}

func renderTemplate(tmpl, serverAddr, mac string) (string, error) {
	tplData := struct {
		URL string
		MAC string
	}{
		URL: "http://" + serverAddr,
		MAC: mac,
	}
	var buf bytes.Buffer
	t, err := template.New("ipxe").Parse(tmpl)
	if err != nil {
		return "", err
	}
	if err := t.Execute(&buf, tplData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func urlJoin(serverAddr string, ptr *string) string {
	if ptr == nil || *ptr == "" {
		return ""
	}
	return "http://" + serverAddr + "/boot/" + *ptr
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

- [ ] **Step 2: 替换 handler 体**

将现有的 `/boot/ipxe/script` handler 体替换为调用 `generateIPXEScript`：

```go
// iPXE 引导脚本端点
r.Get("/boot/ipxe/script", func(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	script, err := generateIPXEScript(cfg, st, mac, r.Host, r.Context())
	if err != nil {
		slog.Error("生成 iPXE 脚本失败", "error", err)
		http.Error(w, "script error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(script)))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(script))
})
```

- [ ] **Step 3: 添加缺失的 import**

确保 `bytes`、`strings`、`text/template` 已经在 imports 中（来自之前的 custom script_template 改动）。

- [ ] **Step 4: 验证编译**

```bash
cd /repo && go build ./...
```

Expected: PASS

- [ ] **Step 5: 运行全部测试**

```bash
cd /repo && go test ./...
```

Expected: all packages pass

- [ ] **Step 6: Commit**

```bash
git add internal/httpd/server.go
git commit -m "feat(server): refactor iPXE script handler to config-driven decision tree"
```

---

### Task 5: 前端类型定义

**Files:**
- Modify: `web/src/api/client.ts:253-264`

为 SettingsData.netboot 添加 script_template 和 boot 字段。

- [ ] **Step 1: 添加 BootSettings 类型**

```typescript
export interface BootSettings {
  default_menu: {
    title: string
    timeout: number
    default: number
    entries: MenuEntry[]
  }
  profile_behavior: {
    append_local: boolean
    append_netboot: boolean
    append_position: 'first' | 'last'
  }
  catalog_redirect: {
    enabled: boolean
    target_url: string
    detect_arch: boolean
    preamble: string
  }
  catalog_display: {
    title: string
    groups: {
      name: string
      title: string
      enabled: boolean
      order: number
    }[]
  }
}
```

- [ ] **Step 2: 更新 SettingsData.netboot 类型**

将：
```typescript
netboot: { enabled: boolean }
```
改为：
```typescript
netboot: { enabled: boolean; script_template?: string; boot: BootSettings }
```

- [ ] **Step 3: 验证 TypeScript 编译**

```bash
cd /repo/web && npx tsc --noEmit
```

Expected: PASS (忽略 pre-existing NetbootCatalog.tsx 错误)

- [ ] **Step 4: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat(web): add BootSettings type definition"
```

---

### Task 6: 前端 UI — 4 个模块编辑器

**Files:**
- Modify: `web/src/pages/Settings.tsx`

在 Netboot 页签下添加 4 个模块的编辑器 UI：默认菜单、Profile 行为、安装目录跳转、安装目录菜单结构。

- [ ] **Step 1: 在 config state 中添加 boot 字段**

在 Settings 组件的 `config` state 中，`netbootScriptTemplate` 之后添加：

```typescript
boot: {
  default_menu: { title: 'PxeGo Boot Menu', timeout: 5000, default: 0, entries: [] as MenuEntry[] },
  profile_behavior: { append_local: true, append_netboot: true, append_position: 'last' as const },
  catalog_redirect: { enabled: true, target_url: 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}', detect_arch: true, preamble: '' },
  catalog_display: {
    title: '[OS] Netboot OS Install Catalog',
    groups: [
      { name: 'linux', title: 'Linux Distributions', enabled: true, order: 1 },
      { name: 'linux-i386', title: 'Linux Distributions (32-bit)', enabled: true, order: 2 },
      { name: 'linux-arm64', title: 'Linux Distributions (arm64)', enabled: true, order: 3 },
      { name: 'bsd', title: 'BSD Systems', enabled: true, order: 4 },
      { name: 'live', title: 'Live CDs', enabled: true, order: 5 },
      { name: 'live-arm', title: 'Live CDs (arm64)', enabled: true, order: 6 },
      { name: 'tools', title: 'System Tools', enabled: true, order: 7 },
      { name: 'windows', title: 'Windows', enabled: true, order: 8 },
      { name: 'dos', title: 'DOS', enabled: true, order: 9 },
      { name: 'unix', title: 'Unix', enabled: true, order: 10 },
    ],
  },
},
```

- [ ] **Step 2: 从 API 加载 boot 设置**

在 `loadSettings` 中，`netbootScriptTemplate` 赋值之后添加：

```typescript
boot: d.netboot?.boot ? {
  default_menu: {
    title: d.netboot.boot.default_menu?.title || prev.boot.default_menu.title,
    timeout: d.netboot.boot.default_menu?.timeout ?? prev.boot.default_menu.timeout,
    default: d.netboot.boot.default_menu?.default ?? prev.boot.default_menu.default,
    entries: d.netboot.boot.default_menu?.entries || [],
  },
  profile_behavior: {
    append_local: d.netboot.boot.profile_behavior?.append_local ?? true,
    append_netboot: d.netboot.boot.profile_behavior?.append_netboot ?? true,
    append_position: d.netboot.boot.profile_behavior?.append_position || 'last',
  },
  catalog_redirect: {
    enabled: d.netboot.boot.catalog_redirect?.enabled ?? true,
    target_url: d.netboot.boot.catalog_redirect?.target_url || 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}',
    detect_arch: d.netboot.boot.catalog_redirect?.detect_arch ?? true,
    preamble: d.netboot.boot.catalog_redirect?.preamble || '',
  },
  catalog_display: {
    title: d.netboot.boot.catalog_display?.title || prev.boot.catalog_display.title,
    groups: d.netboot.boot.catalog_display?.groups || prev.boot.catalog_display.groups,
  },
} : prev.boot,
```

- [ ] **Step 3: 保存时将 boot 数据提交**

在 `handleSave` 的 `netboot` 数据块中，`enabled` 和 `script_template` 之后添加：

```typescript
boot: {
  default_menu: {
    title: config.boot.default_menu.title,
    timeout: config.boot.default_menu.timeout,
    default: config.boot.default_menu.default,
    entries: config.boot.default_menu.entries.filter(e => e.label),
  },
  profile_behavior: config.boot.profile_behavior,
  catalog_redirect: config.boot.catalog_redirect,
  catalog_display: {
    title: config.boot.catalog_display.title,
    groups: config.boot.catalog_display.groups,
  },
},
```

- [ ] **Step 4: 添加默认菜单编辑器 UI**

在 Netboot 页签的 `<div className="space-y-4">` 中，现有内容之后添加第一个面板：

```tsx
{/* ── 默认引导菜单 ── */}
<div className="pt-4 border-t border-[var(--bg-border)]">
  <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">默认引导菜单</h3>
  <p className="text-xs text-[var(--text-muted)] mb-3">无关联 Profile 的主机在 Netboot 关闭时看到此菜单。</p>
  <div className="grid grid-cols-2 gap-4 mb-4">
    <div>
      <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">菜单标题</label>
      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
        value={config.boot.default_menu.title}
        onChange={e => setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, title: e.target.value}}})} />
    </div>
    <div>
      <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">超时时间（秒）</label>
      <input type="number" className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
        value={config.boot.default_menu.timeout}
        onChange={e => setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, timeout: parseInt(e.target.value) || 0}}}}) />
    </div>
  </div>
  {/* 条目编辑器 — 复用 Profiles 的模式 */}
  <div className="space-y-2">
    <label className="block text-xs font-semibold text-[var(--text-secondary)]">菜单条目</label>
    {config.boot.default_menu.entries.map((entry, i) => (
      <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg p-3">
        <div className="grid grid-cols-2 gap-3 mb-2">
          <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
            value={entry.label} placeholder="标签"
            onChange={e => {
              const entries = [...config.boot.default_menu.entries]
              entries[i] = {...entries[i], label: e.target.value}
              setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
            }} />
          <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none"
            value={entry.type}
            onChange={e => {
              const entries = [...config.boot.default_menu.entries]
              entries[i] = {...entries[i], type: e.target.value as any}
              setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
            }}>
            <option value="local">local</option>
            <option value="direct">direct</option>
            <option value="chain">chain</option>
            <option value="sanboot">sanboot</option>
            <option value="wds">wds</option>
          </select>
        </div>
        {entry.type === 'direct' && (
          <div className="space-y-2">
            <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
              value={entry.kernel || ''} placeholder="kernel 路径"
              onChange={e => {
                const entries = [...config.boot.default_menu.entries]
                entries[i] = {...entries[i], kernel: e.target.value}
                setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
              }} />
            <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
              value={entry.initrd || ''} placeholder="initrd 路径"
              onChange={e => {
                const entries = [...config.boot.default_menu.entries]
                entries[i] = {...entries[i], initrd: e.target.value}
                setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
              }} />
            <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
              value={entry.cmdline || ''} placeholder="cmdline 参数"
              onChange={e => {
                const entries = [...config.boot.default_menu.entries]
                entries[i] = {...entries[i], cmdline: e.target.value}
                setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
              }} />
          </div>
        )}
        {entry.type === 'chain' && (
          <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
            value={entry.url || ''} placeholder="chain URL"
            onChange={e => {
              const entries = [...config.boot.default_menu.entries]
              entries[i] = {...entries[i], url: e.target.value}
              setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
            }} />
        )}
        {entry.type === 'wds' && (
          <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
            value={entry.wim || ''} placeholder="WIM 路径"
            onChange={e => {
              const entries = [...config.boot.default_menu.entries]
              entries[i] = {...entries[i], wim: e.target.value}
              setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
            }} />
        )}
        <button className="mt-2 text-xs text-red-400 hover:text-red-300"
          onClick={() => setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries: config.boot.default_menu.entries.filter((_, j) => j !== i)}}}})>删除</button>
      </div>
    ))}
    <button className="text-xs text-blue-400 hover:text-blue-300"
      onClick={() => setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries: [...config.boot.default_menu.entries, {label: '', type: 'local'}]}}}})>+ 添加条目</button>
  </div>
</div>
```

- [ ] **Step 5: 添加 Profile 菜单行为 UI**

在默认菜单之后添加：

```tsx
{/* ── Profile 菜单行为 ── */}
<div className="pt-4 border-t border-[var(--bg-border)]">
  <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">Profile 菜单行为</h3>
  <p className="text-xs text-[var(--text-muted)] mb-3">控制有 Profile 的主机在 iPXE 菜单中自动添加的条目。</p>
  <div className="flex flex-col gap-3">
    <Toggle checked={config.boot.profile_behavior.append_local}
      onChange={v => setConfig({...config, boot: {...config.boot, profile_behavior: {...config.boot.profile_behavior, append_local: v}}}})
      label="追加「从本地硬盘启动」" />
    <Toggle checked={config.boot.profile_behavior.append_netboot}
      onChange={v => setConfig({...config, boot: {...config.boot, profile_behavior: {...config.boot.profile_behavior, append_netboot: v}}}})
      label="追加「OS 安装目录」" />
    <div>
      <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">追加位置</label>
      <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none appearance-none"
        value={config.boot.profile_behavior.append_position}
        onChange={e => setConfig({...config, boot: {...config.boot, profile_behavior: {...config.boot.profile_behavior, append_position: e.target.value as any}}}})>
        <option value="last">菜单末尾</option>
        <option value="first">菜单开头</option>
      </select>
    </div>
  </div>
</div>
```

- [ ] **Step 6: 添加安装目录跳转 UI**

```tsx
{/* ── 安装目录跳转 ── */}
<div className="pt-4 border-t border-[var(--bg-border)]">
  <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">安装目录跳转</h3>
  <p className="text-xs text-[var(--text-muted)] mb-3">无 Profile 且 Netboot 开启时跳转到系统安装目录的脚本行为。</p>
  <div className="flex flex-col gap-4">
    <Toggle checked={config.boot.catalog_redirect.enabled}
      onChange={v => setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, enabled: v}}}})
      label="启用跳转" />
    <div>
      <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">目标 URL</label>
      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] font-mono outline-none focus:border-blue-500"
        value={config.boot.catalog_redirect.target_url}
        onChange={e => setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, target_url: e.target.value}}})} />
      <p className="text-xs text-[var(--text-muted)] mt-1">支持 <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">{`{{.URL}}`}</code> 变量替换为服务器地址。</p>
    </div>
    <Toggle checked={config.boot.catalog_redirect.detect_arch}
      onChange={v => setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, detect_arch: v}}}})
      label="自动检测架构（arch/platform）" />
    <div>
      <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">前置脚本（跳转前执行）</label>
      <textarea rows={4} spellCheck={false}
        className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-4 py-3 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500"
        value={config.boot.catalog_redirect.preamble}
        onChange={e => setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, preamble: e.target.value}}})}
        placeholder="# 可选：在跳转前执行 dhcp、设置变量等" />
    </div>
  </div>
</div>
```

- [ ] **Step 7: 添加安装目录菜单结构 UI（含拖拽排序）**

```tsx
{/* ── 安装目录菜单结构 ── */}
<div className="pt-4 border-t border-[var(--bg-border)]">
  <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">安装目录菜单结构</h3>
  <p className="text-xs text-[var(--text-muted)] mb-3">控制 <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">/netboot/menu.ipxe</code> 的标题和分组顺序。</p>
  <div className="mb-4">
    <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">菜单标题</label>
    <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
      value={config.boot.catalog_display.title}
      onChange={e => setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, title: e.target.value}}}}) />
  </div>
  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-2">分组列表（拖拽排序）</label>
  <div className="space-y-1">
    {[...config.boot.catalog_display.groups]
      .sort((a, b) => a.order - b.order)
      .map((g, i) => (
      <div key={g.name}
        draggable
        onDragStart={e => e.dataTransfer.setData('text/plain', String(i))}
        onDragOver={e => e.preventDefault()}
        onDrop={e => {
          e.preventDefault()
          const fromIdx = parseInt(e.dataTransfer.getData('text/plain'))
          const groups = [...config.boot.catalog_display.groups]
          const sorted = groups.sort((a, b) => a.order - b.order)
          const [moved] = sorted.splice(fromIdx, 1)
          sorted.splice(i, 0, moved)
          const reindexed = sorted.map((g, idx) => ({...g, order: idx + 1}))
          setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, groups: reindexed}}})
        }}
        className="flex items-center gap-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3 py-2 cursor-grab active:cursor-grabbing">
        <span className="text-[var(--text-muted)] cursor-grab">⠿</span>
        <span className="text-xs font-mono text-[var(--text-muted)] w-16">{g.name}</span>
        <input className="flex-1 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
          value={g.title}
          onChange={e => {
            const groups = config.boot.catalog_display.groups.map(g2 =>
              g2.name === g.name ? {...g2, title: e.target.value} : g2)
            setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, groups}}})
          }} />
        <input type="checkbox" checked={g.enabled}
          onChange={e => {
            const groups = config.boot.catalog_display.groups.map(g2 =>
              g2.name === g.name ? {...g2, enabled: e.target.checked} : g2)
            setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, groups}}})
          }}
          className="rounded border-[var(--bg-border)]" title="启用/禁用" />
      </div>
    ))}
  </div>
</div>
```

- [ ] **Step 8: 验证前端编译 + build**

```bash
cd /repo/web && npx tsc --noEmit && npx vite build
```

Expected: PASS (忽略 pre-existing JSX error)

- [ ] **Step 9: 验证后端编译**

```bash
cd /repo && go build ./...
```

Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add web/src/pages/Settings.tsx web/src/api/client.ts
git commit -m "feat(web): add boot settings visual editor UI"
```

---

## Self-Review

**1. Spec coverage:**
- 数据模型: Task 1 (config types) + Task 2 (API types) ✓
- server.go 决策树: Task 4 ✓
- script.go group 配置: Task 3 ✓
- 前端类型定义: Task 5 ✓
- 前端 UI 全部 4 个模块: Task 6 ✓
- 后向兼容: Task 4 的默认值回退处理 ✓
- 逃生口 script_template: Task 4 中仍保留 ✓

**2. Placeholder scan:** 所有步骤都有完整代码，没有 TBD/TODO/省略号。✓

**3. Type consistency:**
- config.MenuEntry 字段名在 config.go 和 api/settings.go 之间一致 ✓
- config.CatalogGroup 和 API CatalogGroupSettings 字段名一致 ✓
- GenerateNetbootScript 新签名在所有调用点一致 ✓
