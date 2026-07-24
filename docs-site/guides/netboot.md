# 网络启动目录（Netboot）

> OS 安装目录菜单、覆盖层、应答文件模板与安装任务管理。

**相关文档**: [引导配置](boot-config.md) | [OS 镜像管理](os-images.md) | [Web UI 指南](web-ui.md)

---

## OS 目录菜单

PxeLab 内置 OS 安装目录，集成 netboot.xyz 的发行版索引，支持 10 个分组：

| 分组 | 内容 |
|------|------|
| Linux Distributions | x86_64 Linux 发行版 |
| Linux Distributions (32-bit) | 32 位 Linux |
| Linux Distributions (arm64) | ARM64 Linux |
| BSD Systems | FreeBSD / OpenBSD 等 |
| Live CDs | 图形化 Live 环境 |
| Live CDs (arm64) | ARM64 Live 环境 |
| System Tools | 系统工具 / 救援镜像 |
| Windows | Windows PE / 安装 |
| DOS | DOS 引导 |
| Unix | 其他 Unix 系统 |

Web UI：**基础配置 → 服务配置 → Netboot 目录**（或侧边栏底部 **设置 → Netboot → 目录菜单结构**）

- 启用/禁用分组
- 自定义显示标题
- 拖拽排序分组顺序

---

## 覆盖层（Overlay）

为特定发行版定制引导参数，不影响默认配置：

- API：`PUT /api/v1/netboot/overlays/{distro}`
- 覆盖 kernel 参数、initrd 参数等
- 按发行版独立配置

---

## 应答文件模板

自动化安装的应答文件管理：

- **预设模板**：常见发行版的安装应答文件
- **自定义模板**：创建、编辑、验证
- **版本管理**：保存历史版本，支持回滚
- **预览**：生成最终应答文件预览
- **验证**：语法和格式验证

API：`/api/v1/netboot/answer-templates`

---

## 安装任务

跟踪和管理网络安装任务：

- 创建安装任务（指定发行版、目标、应答文件）
- 查询任务状态
- 按 MAC 地址查询任务：`GET /api/v1/netboot/task/by-mac/{mac}`
- 获取应答文件：`GET /api/v1/netboot/answer/{task_id}`

---

## 本地缓存

侧边栏底部 **设置 → Netboot** 中启用**本地缓存**（默认开启）：

- 缓存下载的引导文件到磁盘
- 加快重复引导速度
- 缓存统计：`GET /api/v1/netboot/cache-stats`
- Web UI 实时显示缓存路径和磁盘占用
