# PxeGo 实施计划评审报告

评审日期：2026-06-15
评审范围：`C:\Users\darre\.claude\plans\flickering-dazzling-pillow.md`

---

## 1. 架构合理性：✅ 整体良好，有 3 个关注点

### 优点
- **模块边界清晰**：协议层（dhcp/tftp/dns）→ 基础设施层（eventbus/store）→ 表现层（api/httpd/web）分层合理
- **Server 接口设计**：`Start(ctx)/Stop(ctx)` 统一生命周期管理，便于 graceful shutdown
- **BootFileServer 统一根目录**：TFTP 和 HTTP 共享同一文件源，避免文件不一致

### 关注点

| # | 问题 | 建议 |
|---|------|------|
| 1 | **`internal/pxego/` vs `internal/app/`** | 包名 `pxego` 和项目名重合，在 `internal/pxego/app.go` 中 import 会写成 `pxego "pxego/internal/pxego"` 显得冗余。建议改名为 `internal/app/` |
| 2 | **models 和 store 的职责重叠** | `internal/models/` 定义了 `Host` 等 domain struct，`internal/store/models.go` 又定义 GORM model structs。如果没有区别不如合并到 `store/` 中 |
| 3 | **HTTP 服务名 `httpd`** | `internal/httpd/` 和 `net/http` 容易搞混。建议改为 `internal/server/http/` 或 `internal/web/` |

---

## 2. 依赖选择：✅ 合理，2 个补充

### 已选依赖评价

| 依赖 | 评价 |
|------|------|
| `cobra + viper` | 行业标准，正确 |
| `chi/v5` | 轻量，兼容 `http.Handler`，正确 |
| `insomniacslk/dhcp` | 最成熟的 Go DHCP 库，正确 |
| `pin/tftp` | 可靠，但注意该项目归档状态，考虑备选 |
| `miekg/dns` | 标准 DNS 库，正确 |
| `glebarez/sqlite` | pure-Go，CGo-free，跨平台编译关键选择，正确 |
| `log/slog` | Go 1.21 引入的结构化日志，stdlib 零依赖，正确的轻量选择 |

### 需要补充

| 缺失 | 原因 | 推荐 |
|------|------|-----|
| **Wake-on-LAN 库** | API 中有 `POST /hosts/{id}/wake` 但无 WoL 实现 | `github.com/sabrier/wol` 或自行实现（9 字节 magic packet 很简单）|
| **ARP/网络扫描** | IPMI 扫描需要发现 BMC 地址 | `github.com/google/gopacket` 用于发送 ARP 请求 |
| **UUID 库** | 事件/设备标识 | `github.com/google/uuid` 标准库 |
| **嵌入式 iPXE 二进制** | 计划提到提供 iPXE 引导文件但未说明来源 | 需要使用 `go:embed` 嵌入 `undionly.kpxe`、`ipxe.efi` 等预编译二进制文件 |

---

## 3. 阶段排序：✅ 基本合理，但有 1 个风险

### 当前顺序

```
Phase 1: Foundation
Phase 2: DHCP + TFTP (核心 PXE)
Phase 3: HTTP + iPXE Engine
Phase 4: PXELinux 兼容
Phase 5: REST API + IPMI
Phase 6: 前端 SPA
Phase 7: DNS + 加固
Phase 8: 服务集成 + 发布
```

### 风险：Phase 6（前端）排得太后

在 Phase 5 完成 API 后，直到 Phase 6 才能看到 UI 效果。这意味着：
- Phase 5 的 API 无法被前端验证（只能手动 curl）
- 前后端接口不一致的风险积压到 Phase 6

**建议**：
- Phase 3 后（HTTP 服务已启动）就搭建一个**极简管理页面**（Go templates，无 React）：能显示状态、事件列表即可
- React SPA 作为 Phase 6 增强替换

或者保持原计划，但 Phase 5 结束时必须用 curl 脚本做完整的 API 集成测试。

### 另一个建议：DNS 提到 Phase 3

DNS 代码量小（`miekg/dns` 约 100 行就能跑起来），放在 Phase 7 显得浪费。可以考虑提到 Phase 3 末尾，和 HTTP 服务一起上线。

---

## 4. 风险区域

| 风险 | 等级 | 原因 | 缓解措施 |
|------|------|------|---------|
| **DHCP 端口权限** | ⚠️ 致命 | 端口 67/69/53 在 Linux 上需要 root 或 `CAP_NET_BIND_SERVICE` | 计划已提到 `AmbientCapabilities`，需要确保文档明确告知用户 |
| **Windows 端口权限** | ⚠️ 致命 | Windows 上绑定端口 67/69 需要管理员权限 | 需要管理员令牌或 Windows 服务运行 |
| **glebarez/sqlite 写入冲突** | ⚠️ 重要 | SQLite 并发写入会锁库，DHCP 高频事件写入可能碰撞 | GORM 连接池设为 1，或使用 WAL 模式 |
| **TFTP 并发性能** | ⚠️ 重要 | `pin/tftp` 的默认配置可能不够优化，大规模 PXE 启动时可能超时 | 进行压力测试，必要时限制并发传输数 |
| **iPXE 模板注入** | ⚠️ 重要 | 配置文件/用户输入直接进 `text/template` 可能导致意外渲染 | 模板数据需要转义，不允许用户自定义模板语法 |
| **跨平台 UDP 行为差异** | ⚡ 注意 | Windows 的 UDP 广播行为与 Linux 不同 | 在 Windows 上测试 DHCP 代理模式 |
| **前端构建依赖** | ⚡ 注意 | 构建时需要 Node.js 构建前端 | 在 `.goreleaser.yaml` 的 before hook 中确保 npm ci + build |
| **pxelinux.cfg 文件名匹配** | ⚡ 注意 | PXELinux 配置文件按 IP/MAC 十六进制匹配，规则复杂 | 需要完整实现 PXELinux 配置解析规范 |


## 5. 与 Tiny PXE Server 功能对照

| Tiny PXE Server 功能 | 本计划覆盖 | 备注 |
|----------------------|-----------|------|
| DHCP server | ✅ | full/proxy/hybrid 三种模式 |
| DHCP proxy | ✅ | 通过 option 60 检测 |
| TFTP server | ✅ | 统一 BootFileServer |
| HTTP boot (winpe) | ✅ | /boot/ 文件服务 + iPXE |
| DNS server | ✅ | Phase 7 |
| PXELinux | ✅ | AST 翻译方案 |
| gPXE/iPXE | ✅ | 模板引擎 |
| WinPE boot | ⚠️ 部分 | HTTP 文件服务可以支持，但缺少 WinPE 专用优化 |
| WDS (Windows Deployment) | ❌ 缺失 | Tiny PXE 支持 WDS 仿真，本计划未提及 |
| BIOS/UEFI 自动检测 | ❌ 缺失 | 需要根据不同架构提供不同引导文件 |
| Proxy DHCP (port 4011) | ⚠️ 部分 | 计划提到在 67 端口统一处理，缺 PXE 标准的 4011 端口代理 |

**需补充的功能**：
1. **WDS 仿真**：Windows 部署场景需要，可选但重要
2. **ProxyDHCP on port 4011**：PXE 规范要求 proxyDHCP 也可以监听 4011 端口
3. **BIOS/UEFI 架构检测**：通过 DHCP option 93 检测客户端架构，自动返回正确的引导文件
4. **日志轮转/日志文件**：运维工具需要持久化日志
5. **配置热重载**：修改配置文件后无需重启服务

---

## 6. 跨平台注意事项

### Linux ✅ 最佳
- systemd 服务 + Capabilities 权限控制
- `CAP_NET_BIND_SERVICE` + `CAP_NET_RAW` 覆盖所有需求

### Windows ⚠️
- **服务模式**：使用 `golang.org/x/sys/windows/svc` 是正确的
- **端口 67/69**：需要管理员权限运行
- **路径格式**：配置中的路径需要使用平台判断
- **Windows Defender**：UDP 端口监听可能触发防火墙弹窗，需要安装时添加规则

### macOS ⚠️
- **launchd** 配置需要签名才能开机自启
- **端口 53**（DNS）在 macOS 上通常被系统解析器占用，建议默认禁用 DNS 或使用非标准端口
- **SIP** 可能阻止某些端口绑定

### 建议
在 `cmd/pxego/` 下为每个平台提供专门的 service 文件，平台检测通过 Go build tags 实现：

```
cmd/pxego/
├── main.go                    # 通用入口
├── service_windows.go         //go:build windows
├── service_linux.go           //go:build linux
├── service_darwin.go          //go:build darwin
└── path_unix.go               //go:build unix
└── path_windows.go            //go:build windows
```

---

## 7. 总体评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 架构设计 | 8/10 | 模块划分清晰，但包名有小问题 |
| 依赖选择 | 9/10 | 主流库选择正确，缺 WoL 和 gopacket |
| 阶段排序 | 7/10 | 前端排太后，DNS 排太后，WDS 未覆盖 |
| 风险识别 | 6/10 | 计划中缺少风险章节，需补充 |
| 跨平台 | 7/10 | 服务集成考虑全面，但缺少平台差异测试 |
| Tiny PXE 覆盖 | 7/10 | 核心功能全，缺 WDS 和 4011 端口 |

**总体：建议补充风险章节和缺失功能后再进入实施。**
