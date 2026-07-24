# 服务配置（Service Config）

> 二级子导航包含 6 个服务配置页面，点击「服务配置」后在右侧展开子页面列表。

**相关文档**: [DHCP 配置](dhcp.md) | [TFTP 服务](../reference/tftp.md) | [DNS 服务](../reference/dns.md) | [NFS 服务](../reference/nfs.md) | [架构映射](../reference/boot-settings.md)

---

## DHCP（/services/dhcp）

- **子网配置**：每个子网独立设置 DHCP 模式（full/proxy/hybrid/off）、地址池范围、网关、DNS、租期
- **预留管理（Tab）**：IP + MAC 绑定，冲突检测，仅 full 模式子网可用
- **租约列表（Tab）**：查看当前活跃租约，支持删除和批量清理

详见 [DHCP 配置指南](dhcp.md)。

## DNS（/services/dns）

- 上游 DNS 配置
- 本地域名设置
- DNS 记录管理（A / AAAA / CNAME）
- 子网感知解析配置

详见 [DNS 服务参考](../reference/dns.md)。

## NFS（/services/nfs）

- 多挂载点管理（动态卡片列表）
  - 添加/删除挂载点
  - 每个挂载点：标签、导出路径、本地目录、只读开关、IP/CIDR 白名单
- NFS 连接状态：当前连接数、客户端列表
- 端口配置

详见 [NFS 服务参考](../reference/nfs.md)。

## TFTP（/services/tftp）

- 端口和超时设置
- 引导文件管理（Tab 页签）：iPXE / PXELinux / GRUB2 三栏展示架构对应关系
- 文件管理：上传、删除引导文件，表格展示（名称、大小、修改时间、MD5）

详见 [TFTP 服务参考](../reference/tftp.md)。

## Boot Settings（/boot-settings）

架构映射管理页面：

- **引导文件健康检查**：检查所有架构的引导文件是否存在、大小是否正常
- **架构映射表**：10 种架构的配置表格
  - 列：架构名称、AL 码、NBP 类型（iPXE/PXELinux/GRUB2 下拉切换）、引导文件名、Secure Boot 支持状态
  - NBP 切换时自动更新引导文件名
  - ARM64 + PXELinux 自动回退提示
- **iPXE 脚本设置**：自定义 iPXE 脚本（模板变量 `{`{`.URL}` / `{`{`.MAC}`）
- **操作**：保存、恢复默认值

详见 [架构映射与 Secure Boot](../reference/boot-settings.md)。

## Netboot 目录（/netboot-catalog）

OS 安装目录管理：

- 发行版列表：显示所有可用发行版及其版本、架构
- 分组管理：10 个内置分组的启用/禁用、标题编辑、拖拽排序
- 缓存统计：缓存路径、文件数、磁盘占用

详见 [网络启动目录](netboot.md)。
