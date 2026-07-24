# 架构优势

> PxeLab 的技术架构设计亮点。

**相关文档**: [架构概述](architecture.md) | [优势能力](advantages.md) | [配置文件参考](reference/config-file.md)

---

## 整体架构

```
┌─────────────────────────────────────────────────────┐
│                    PxeLab Binary                     │
├─────────────┬─────────────┬─────────────┬───────────┤
│  Web UI     │  REST API   │  Services   │  Store    │
│  (React SPA)│  (chi)      │  Manager    │  (SQLite) │
├─────────────┴─────────────┴─────────────┴───────────┤
│              Service Manager                        │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐         │
│  │DHCP │ │TFTP │ │HTTP │ │DNS  │ │NFS  │         │
│  └─────┘ └─────┘ └─────┘ └─────┘ └─────┘         │
├─────────────────────────────────────────────────────┤
│              Event Bus (Pub/Sub)                    │
└─────────────────────────────────────────────────────┘
```

---

## 1. Go 单二进制部署

### 为什么选择 Go？

| 特性 | 优势 |
|------|------|
| **静态编译** | 无运行时依赖，部署简单 |
| **跨平台** | 一份代码，编译出 Windows/Linux/macOS 二进制 |
| **并发模型** | Goroutine 高效处理网络连接 |
| **标准库** | 内置 HTTP、DNS、NFS 等网络服务支持 |
| **性能** | 接近 C 的性能，远超脚本语言 |

### 嵌入式资源

所有前端资源（HTML/CSS/JS）和引导文件（iPXE 二进制、模板）都嵌入到二进制文件中：

```go
//go:embed webdist/*
var webDist embed.FS

//go:embed ipxe/*
var ipxeBinaries embed.FS
```

**优势**：
- 无需单独部署前端服务
- 无需维护文件系统路径
- 升级只需替换一个文件
- 无文件权限问题

---

## 2. 服务管理器（Service Manager）

统一管理所有服务的生命周期：

```
ServiceManager
├── StartAll()     # 启动所有服务
├── StopAll()      # 停止所有服务
├── RestartAll()   # 重启所有服务
├── Start(name)    # 启动单个服务
├── Stop(name)     # 停止单个服务
└── Status()       # 获取所有服务状态
```

**设计亮点**：
- 每个服务实现统一的 `Service` 接口
- 支持独立启动/停止，互不影响
- 热重载配置，无需重启进程
- 优雅关闭（Graceful Shutdown）

---

## 3. 事件总线（Event Bus）

内部发布/订阅系统，解耦服务间通信：

```go
// 发布事件
eventbus.Publish(Event{
    Type:    "dhcp.lease",
    Payload: LeaseInfo{...},
})

// 订阅事件
eventbus.Subscribe("dhcp.lease", func(e Event) {
    // 处理 DHCP 租约事件
})
```

**支持的事件类型**：
- DHCP 租约事件
- 引导事件（BOOT）
- WOL 唤醒事件
- IPMI/BMC 操作事件
- DNS 查询事件

**优势**：
- 服务间松耦合
- 易于扩展新事件类型
- 支持异步处理
- 日志和审计追踪

---

## 4. 存储接口分离（Store Interface）

数据层采用接口设计，支持多种实现：

```go
type Interface interface {
    HostStore
    ProfileStore
    SettingsStore
    // ... 其他子接口
}

// SQLite 实现（生产环境）
type SQLiteStore struct { ... }

// 内存实现（测试/开发）
type MemoryStore struct { ... }
```

**优势**：
- 测试时使用内存存储，快速且隔离
- 生产环境使用 SQLite，轻量且可靠
- 易于切换到其他数据库（如 PostgreSQL）
- 接口定义清晰，易于扩展

---

## 5. 依赖注入（Dependency Injection）

`main.go` 中显式构造所有服务依赖：

```go
func run() {
    // 显式构造，无全局单例
    store := sqlite.New(dbPath)
    eventBus := eventbus.New()
    dhcpServer := dhcp.New(store, eventBus)
    tftpServer := tftp.New(bootFileServer)
    httpServer := httpd.New(store, ...)
    
    // 服务管理器统一管理
    manager := servicemanager.New(
        dhcpServer,
        tftpServer,
        httpServer,
        dnsServer,
        nfsServer,
    )
    
    // 启动所有服务
    manager.StartAll()
}
```

**优势**：
- 依赖关系清晰可见
- 便于测试和 Mock
- 无隐藏的全局状态
- 易于理解和维护

---

## 6. CSS 变量主题系统

前端使用 CSS 变量实现主题切换：

```css
:root {
    --bg-base: oklch(0.145 0 0);
    --text-primary: oklch(0.985 0 0);
    --accent-blue: oklch(0.546 0.245 262.881);
}

.dark {
    --bg-base: oklch(0.145 0 0);
    --text-primary: oklch(0.985 0 0);
}
```

**优势**：
- 一键切换亮色/暗色主题
- 所有组件自动适配
- 无需维护两套样式
- 用户偏好持久化

---

## 7. 代码分割与懒加载

React 路由级代码分割：

```typescript
const Dashboard = React.lazy(() => import('./pages/Dashboard'))
const Hosts = React.lazy(() => import('./pages/Hosts'))

// 路由配置
<Route path="/dashboard" element={
    <Suspense fallback={<Skeleton />}>
        <Dashboard />
    </Suspense>
} />
```

**优势**：
- 首屏加载快
- 按需加载，减少初始包体积
- 骨架屏提升用户体验
- 构建工具自动优化

---

## 技术栈总结

| 层级 | 技术 | 说明 |
|------|------|------|
| **后端** | Go 1.23+ | 高性能，静态编译 |
| **前端** | React 19 + TypeScript | 现代化 UI 框架 |
| **样式** | Tailwind CSS 4 | 原子化 CSS |
| **数据库** | SQLite (GORM) | 轻量嵌入式数据库 |
| **路由** | chi | 轻量 HTTP 路由器 |
| **构建** | Vite 6 | 快速前端构建 |
| **打包** | GoReleaser | 自动化发布 |
