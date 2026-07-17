# PxeLab Roadmap

## Planned

### [P2] GitHub Actions 自动化发布 boot 启动器

**目标**：将 `boot/` 目录下的二进制启动器（iPXE、GRUB、PXELINUX 等）从 Git 仓库移到 GitHub Releases，通过 CI/CD 自动化管理。

**方案**：
1. 创建 GitHub Actions workflow，触发条件为推送 tag（如 `v1.0.0`）
2. Workflow 自动执行：
   - 编译 Go 多平台二进制
   - 打包 `boot/` 启动器为压缩包
   - 上传到 GitHub Release 作为 release assets
3. 代码中硬编码 `BOOT_VERSION` 常量
4. 程序启动时检查 `boot/` 目录，缺少文件则从 Release 下载

**需要移动的文件**：
- `boot/*.efi` (iPXE, GRUB)
- `boot/*.pxe` (iPXE PXE)
- `boot/pxelinux.*` (PXELINUX)
- `boot/ldlinux.*`
- `boot/memdisk`
- `boot/menu.c32`
- `boot/undionly.kpxe`

**保留的文件**（配置文件，体积小）：
- `boot/grub.cfg`
- `boot/grub.cfg.common`
- `boot/pxelinux.cfg/default`

**收益**：
- 仓库体积从 ~75MB 降至 ~10MB
- 用户下载 Release 包即可直接使用
- 启动器版本可独立更新

**状态**：⏳ 待实施

---

### [P3] 开源版 vs 商业版功能划分

**目标**：规划开源核心版和商业增强版的功能边界。

**定位**：
- **开源版 = PXE 底座**：功能完整无限制，快速装机，个人/小团队免费使用
- **商业版 = 企业级扩展**：基于底座构建高级能力，面向企业 IT 部门和 IDC/云厂商

#### 开源版（底座）— 免费，功能完整

| 模块 | 功能 |
|------|------|
| 核心服务 | DHCP / TFTP / PXE 服务 |
| 启动器 | iPXE + PXELINUX + GRUB |
| 管理界面 | Web UI 全功能 |
| 主机管理 | 增删改查、分组 |
| 启动菜单 | 可视化编辑 |
| 网络唤醒 | WOL 基础功能 |
| 命令行 | CLI 管理工具 |
| 部署方式 | 单节点 |

#### 商业版（企业版）— 付费增强

| 方向 | 功能 |
|------|------|
| **多节点管理** | 一个控制台管多台 PxeLab 服务器 |
| **高可用** | 主备切换、故障自动恢复 |
| **安全合规** | LDAP/AD 集成、RBAC 细粒度权限、审计日志 |
| **自动化运维** | 批量装机工作流、与 CMDB/ITSM 集成 |
| **高级报告** | 装机统计、资产报表、导出 |
| **API 增强** | Webhook、API 限流、第三方集成 |
| **技术支持** | 优先响应、定制开发 |

#### 商业模式

**三者结合，灵活定价**：

1. **免费版**：单节点，功能完整，个人/小团队够用
2. **按规模付费**：多节点、高并发场景，按节点数收费
3. **订阅制**：含技术支持 + 持续更新，按年/月订阅

**状态**：✅ 已讨论

---

## Backlog

- [ ] Docker 容器化部署
- [ ] 多语言国际化完善
- [ ] 性能监控仪表盘
- [ ] 自动化测试覆盖率提升
