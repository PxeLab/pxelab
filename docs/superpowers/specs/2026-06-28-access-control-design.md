# 访问控制 — 黑白名单设计

## 概述

在 PxeGo 的 DHCP handler 中增加基于 MAC 的黑名单（全局）和白名单（子网级）过滤。黑名单 MAC 在 DHCP 层面静默忽略；开启了白名单的子网只响应已知 MAC。

## Config 模型

### GlobalConfig — 新增字段

```yaml
global:
  whitelist_enabled: false   # 全局白名单开关
```

### SubnetConfig — 新增字段

```yaml
interfaces:
  - name: eth0
    subnets:
      - cidr: 192.168.1.0/24
        whitelist_enabled: true   # 子网级白名单开关
```

### 种子条目（顶层，仅启动时导入）

```yaml
blacklist:
  - mac: "00:11:22:33:44:55"
    reason: "被盗设备"

whitelist:
  - mac: "AA:BB:CC:DD:EE:FF"
    reason: "管理机"
    subnet: "192.168.1.0/24"
```

种子条目在首次启动时写入数据库（幂等——MAC 已存在则跳过）。运行时的增删只操作数据库。

## 数据模型（SQLite, GORM）

```go
type BlacklistEntry struct {
    ID        uint      `gorm:"primaryKey"`
    MAC       string    `gorm:"uniqueIndex;size:17"`
    Reason    string    `gorm:"size:255"`
    Source    string    `gorm:"size:32;default:db"`   // "db" | "seed"
    CreatedAt time.Time
    UpdatedAt time.Time
}

type WhitelistEntry struct {
    ID         uint      `gorm:"primaryKey"`
    MAC        string    `gorm:"index:idx_mac_subnet,unique;size:17"`
    SubnetCIDR string    `gorm:"index:idx_mac_subnet,unique;size:43"`
    Reason     string    `gorm:"size:255"`
    Source     string    `gorm:"size:32;default:db"`
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

### Store 接口新增

```go
type BlacklistStore interface {
    ListBlacklist(ctx context.Context) ([]models.BlacklistEntry, error)
    CreateBlacklist(ctx context.Context, entry *models.BlacklistEntry) error
    DeleteBlacklist(ctx context.Context, id uint) error
    IsBlacklisted(ctx context.Context, mac string) (bool, error)
}

type WhitelistStore interface {
    ListWhitelist(ctx context.Context) ([]models.WhitelistEntry, error)
    CreateWhitelist(ctx context.Context, entry *models.WhitelistEntry) error
    DeleteWhitelist(ctx context.Context, id uint) error
    IsWhitelisted(ctx context.Context, mac, subnetCIDR string) (bool, error)
}
```

## DHCP Handler — 拦截链

在 `Handle()` 中匹配到子网后、`switch mt` 之前插入：

1. **黑名单检查** → `store.IsBlacklisted(mac)` → 是则记录日志 + 静默返回
2. **全局白名单检查** → `config.Global.WhitelistEnabled` → 若开启则 `store.IsWhitelisted(mac, subnetCfg.CIDR)` → 未命中则记录日志 + 返回
3. **子网白名单检查** → `subnetCfg.WhitelistEnabled` → 若开启则 `store.IsWhitelisted(mac, subnetCfg.CIDR)` → 未命中则记录日志 + 返回
4. 继续正常处理 DISCOVER/REQUEST

所有被拒绝的请求都会记录一条 info 级别的事件到事件总线，方便排查。

## 启动时种子导入

在 `cmd/pxego/main.go` 中，`st.Migrate()` 之后、服务启动之前：

```go
for _, entry := range cfg.BlacklistSeeds {
    exists, _ := st.IsBlacklisted(ctx, entry.MAC)
    if !exists {
        st.CreateBlacklist(ctx, &models.BlacklistEntry{MAC: entry.MAC, Reason: entry.Reason, Source: "seed"})
    }
}
for _, entry := range cfg.WhitelistSeeds {
    exists, _ := st.IsWhitelisted(ctx, entry.MAC, entry.Subnet)
    if !exists {
        st.CreateWhitelist(ctx, &models.WhitelistEntry{MAC: entry.MAC, SubnetCIDR: entry.Subnet, Reason: entry.Reason, Source: "seed"})
    }
}
```

## API 端点

```
GET    /api/v1/access/blacklist         → 列出所有黑名单条目
POST   /api/v1/access/blacklist         → 添加 { mac, reason? }
DELETE /api/v1/access/blacklist/{id}    → 删除

GET    /api/v1/access/whitelist         → 列出所有白名单（支持 ?subnet= 过滤）
POST   /api/v1/access/whitelist         → 添加 { mac, subnet_cidr, reason? }
DELETE /api/v1/access/whitelist/{id}    → 删除
```

Settings API 扩展：`GET /api/v1/settings` 返回顶层 `whitelist_enabled: bool`；`PUT /api/v1/settings` 接受该字段。

## Web UI

### 导航

「管理」区新增菜单项「访问控制」，路由 `/netboot/access-control`。

### Settings 页

General tab 增加「全局白名单开关」toggle。

### 访问控制页

双栏布局：
- 左栏：黑名单表格（MAC、备注、删除按钮）+ 添加表单
- 右栏：白名单表格（MAC、子网 CIDR、备注、删除按钮）+ 添加表单（含子网 CIDR 选择器）

## 涉及文件

| 层 | 文件 | 改动 |
|-------|------|--------|
| Config | `internal/config/config.go` | 加字段 + MACEntry 结构体 |
| 模型 | `internal/models/access.go` | 新文件 |
| 存储接口 | `internal/store/store.go` | 接口增加 |
| 存储实现 | `internal/store/sqlite.go` | 注册 Migrate |
| 存储实现 | `internal/store/sqlite_access.go` | 新文件 |
| DHCP | `internal/dhcp/handler.go` | 拦截链 |
| API | `internal/api/handler.go` | 路由注册 |
| API | `internal/api/access.go` | 新文件 |
| API | `internal/api/settings.go` | 白名单开关传递 |
| 入口 | `cmd/pxego/main.go` | 种子导入 |
| UI | `web/src/pages/AccessControl.tsx` | 新文件 |
| UI | `web/src/App.tsx` | 路由 |
| UI | `web/src/components/layout/AppShell.tsx` | 菜单项 |
| UI | `web/src/pages/Settings.tsx` | 白名单开关 toggle |
| UI | `web/src/api/client.ts` | API 方法 |
