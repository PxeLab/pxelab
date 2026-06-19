# iPXE 引导行为可视化管理设计

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task.

**Goal:** 将目前硬编码在 server.go、script.go 中的 iPXE 引导逻辑提取为可配置的数据模型，通过 Web UI 可视化管理

**Architecture:** 在现有 NetbootConfig 下新增 `boot` 字段，包含 default_menu、profile_behavior、catalog_redirect、catalog_display 四个子模块。server.go 的 iPXE 脚本端点改为纯配置驱动。UI 复用 Profiles 的条目编辑器组件。

**Tech Stack:** Go (config + chi + text/template)、React (Settings.tsx + 复用条目编辑器)

---

## 数据模型

```yaml
# config.yaml 新增结构
netboot:
  enabled: true
  script_template: ""          # 非空时完全覆盖所有引导逻辑（逃生口）
  boot:
    default_menu:
      title: "PxeGo Boot Menu"
      timeout: 5000
      default: 0
      entries:
        - label: "Boot from local disk"
          type: local

    profile_behavior:
      append_local: true        # Profile 菜单末尾追加"从本地硬盘启动"
      append_netboot: true      # Profile 菜单末尾追加"OS 安装目录"
      append_position: "last"   # first | last

    catalog_redirect:
      enabled: true
      target_url: "http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}"
      detect_arch: true         # 是否在跳转前加入架构检测
      preamble: ""              # 跳转前的前置脚本

    catalog_display:
      title: "[OS] Netboot OS Install Catalog"
      groups:
        - name: linux
          title: "Linux Distributions"
          order: 1
          enabled: true
        - name: linux-i386
          title: "Linux Distributions (32-bit)"
          order: 2
          enabled: true
        - name: linux-arm64
          title: "Linux Distributions (arm64)"
          order: 3
          enabled: true
        - name: bsd
          title: "BSD Systems"
          order: 4
          enabled: true
        - name: live
          title: "Live CDs"
          order: 5
          enabled: true
        - name: live-arm
          title: "Live CDs (arm64)"
          order: 6
          enabled: true
        - name: tools
          title: "System Tools"
          order: 7
          enabled: true
        - name: windows
          title: "Windows"
          order: 8
          enabled: true
        - name: dos
          title: "DOS"
          order: 9
          enabled: true
        - name: unix
          title: "Unix"
          order: 10
          enabled: true
```

## 模块详述

### 1. Default Menu

与 Profile 的 MenuEntry 类型完全一致。配置存储而非数据库存储。

字段：title, timeout, default, entries[]

Entries 支持类型：local, direct, chain, sanboot, wds

### 2. Profile Behavior

控制有 Profile 的主机的菜单行为。

- `append_local`: 是否在 Profile 菜单末尾追加"Boot from local disk"入口
- `append_netboot`: 是否在 Profile 菜单末尾追加 Netboot 目录入口
- `append_position`: 插入位置，"first"/"last"

### 3. Catalog Redirect

控制无 Profile + Netboot 开启时的跳转行为。

- `target_url`: 支持 `{{.URL}}` 模板变量（服务器基地址）
- `detect_arch`: 为 true 时，自动在脚本开头插入架构检测代码（cpuid、platform 检测）
- `preamble`: 用户自定义前置脚本（在检测后、跳转前执行）

渲染逻辑：
1. detect_arch=true 时插入架构检测
2. 插入 preamble（如果非空）
3. 插入 chain 到 target_url

### 4. Catalog Display

控制 `/netboot/menu.ipxe` 的展示结构。

- groups: 有序列表，每个 group 有 name、title、enabled、order
- order 字段用于前端拖拽排序持久化
- enabled=false 的 group 不渲染任何条目
- 不在配置中的 group name 回退到硬编码标题（后向兼容）

## 后端改动

### 文件清单

| 文件 | 改动内容 |
|------|----------|
| `internal/config/config.go` | NetbootConfig 新增 `Boot BootConfig` |
| `internal/config/config.go` | 新增 BootConfig / DefaultMenuConfig / ProfileBehavior / CatalogRedirect / CatalogDisplay / CatalogGroup |
| `internal/api/settings.go` | SettingsResponse 新增 BootSettings 结构，Get/Update 中读写 |
| `internal/httpd/server.go` | `/boot/ipxe/script` 从硬编码改为配置驱动决策树 |
| `internal/netboot/script.go` | `GenerateNetbootScript()` 接受 CatalogDisplay 参数代替硬编码 groupTitle |
| `internal/netboot/script_test.go` | 更新测试适配新参数 |

### server.go 新决策树

```
func handleIPXEScript(w, r):
  if script_template non-empty:
    render and return

  if host has profile:
    render profile entries
    if profile_behavior.append_local:                        append local entry
    if profile_behavior.append_netboot AND netboot.enabled:  append netboot entry
    return

  if catalog_redirect.enabled:
    script = ""
    if detect_arch: script += arch detection
    if preamble:    script += preamble
    script += chain to target_url
    return

  return default_menu
```

### script.go 改动

`GenerateNetbootScript()` 增加 `groups []CatalogGroup` 参数，替代硬编码的 `groupTitle()` map：

```go
func GenerateNetbootScript(c *Catalog, serverAddr, arch, platform string, groups []config.CatalogGroup) string
```

Group 按 order 排序。不在 groups 列表中的 group name 使用 title 原文显示。enabled=false 的 group 跳过。

## 前端改动

### Settings.tsx Netboot 页签

```
Netboot 页签结构（从上到下）:
├── 启用 OS 安装目录菜单 (Toggle)
├── 自定义 iPXE 脚本 (Textarea, 已有)
│
├── ── 默认引导菜单 ──
│   ├── 菜单标题 (Input)
│   ├── 超时时间 (Input, 秒)
│   ├── 条目列表 (复用 Profiles 的条目编辑器)
│
├── ── Profile 菜单行为 ──
│   ├── 追加"从本地硬盘启动" (Toggle)
│   ├── 追加"OS 安装目录" (Toggle)
│   ├── 追加位置 (Select: 最前 / 最后)
│
├── ── 安装目录跳转 ──
│   ├── 启用跳转 (Toggle)
│   ├── 目标 URL (Input, 支持 {{.URL}})
│   ├── 架构检测 (Toggle)
│   ├── 前置脚本 (Textarea)
│
├── ── 安装目录菜单结构 ──
│   ├── 菜单标题 (Input)
│   ├── 分组列表 (拖拽排序)
│   │   └── 每组: name (只读)、title (可编辑)、enabled (Toggle)、order (拖动)
```

### client.ts

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

### 复用模式

条目编辑器直接复用 Profiles.tsx 中 `updateEntry()` + 条件字段渲染的模式。

拖拽排序实现：使用 HTML5 Drag & Drop API（不引入第三方库，几个可拖动的行即可）。

## 后向兼容

- `catalog_display.groups` 为空时，`GenerateNetbootScript()` 使用现有硬编码 group 标题
- `default_menu.entries` 为空时，server.go 回退到现有的最小菜单
- `catalog_redirect` 默认值保持当前行为（enabled=true, chain to `/netboot/menu.ipxe`）
- `script_template` 优先级最高，完全覆盖所有逻辑，保持逃生口能力

## 自检清单

- [x] 所有硬编码点都有对应的配置项覆盖
- [x] 条目标题不硬编码（group title 可配置）
- [x] Profile 菜单的追加行为可控制
- [x] Netboot 跳转目标可修改
- [x] 后向兼容：配置缺失时回退到现有行为
- [x] 逃生口：script_template 仍然有效
- [x] `append_netboot` 需要同时受 `netboot.enabled` 控制（netboot 关闭时不显示）
- [x] 条目编辑器复用而非重新实现
- [x] 测试覆盖配置驱动的各条路径
