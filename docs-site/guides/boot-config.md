# 引导配置

> iPXE 引导脚本系统、Profile 管理与默认菜单配置。

**相关文档**: [架构映射与 Secure Boot](../reference/boot-settings.md) | [DHCP 配置](dhcp.md) | [网络启动目录](netboot.md)

---

## iPXE 引导脚本系统

PxeLab 的 iPXE 引导采用**配置驱动决策树**设计，无需编写原始 iPXE 脚本，通过 Web 界面可视化配置：

```
客户端请求引导脚本
    │
    ├─ 1. 自定义脚本？ → 有 → 直接返回，忽略所有下方配置
    │
    ├─ 2. 主机有 Profile？ → 有 Profile 且含菜单 → 返回 Profile 菜单
    │
    ├─ 3. 安装目录跳转？ → 已启用 → chain 到 OS 安装目录
    │
    └─ 4. 默认引导菜单 → 返回配置的默认菜单
```

---

## 引导菜单类型

| BootType | 用途 | 示例 |
|----------|------|------|
| `local` | 从本地硬盘启动 | 跳过网络引导 |
| `direct` | 直接加载内核 + initrd | Linux 发行版安装 |
| `chain` | Chain-load 其他引导器 | GRUB2、Windows Boot Manager |
| `wds` | Windows WIM 引导 | Windows PE / 安装 |
| `sanboot` | iSCSI SAN 启动 | 无盘工作站 |

### sanboot 适用场景

| 场景 | 是否适合 | 原因 |
|------|---------|------|
| DOS 启动盘 | ✅ | 启动即运行，不再访问外部介质 |
| Live Linux | ✅ | 内核 + initramfs 自包含 |
| WinPE 维护盘 | ✅ | 进 PE 后通过网络获取工具 |
| Memtest86+ | ✅ | 引导后不读盘 |
| iSCSI LUN 直启 | ✅ | 已安装系统盘 |
| CentOS/RHEL 安装 | ⚠️→❌ | Anaconda 需显式 inst.repo |
| Windows 安装 | ❌ | 需提取 wim + BCD，走 wimboot |

---

## Profile（引导配置文件）

Profile 是绑定到特定主机的引导配置，包含一个引导菜单：

- **创建 Profile**：指定名称、架构、引导类型和参数
- **绑定主机**：将 Profile 绑定到主机 MAC 地址
- **脚本版本管理**：每次修改自动保存版本快照，支持差异对比和回滚
- **从 Netboot 创建**：可从 OS 安装目录一键创建 Profile

每个 Profile 为单引导项，简化配置：

```
Profile: "Install Ubuntu 22.04"
  ├─ 架构: x86_64
  ├─ 类型: direct
  ├─ Kernel: vmlinuz
  ├─ Initrd: initrd.img
  └─ Cmdline: net.ifnames=0 console=tty0
```

---

## 默认引导菜单

当客户端**无关联 Profile** 且**未启用 Netboot 安装目录跳转**时，显示默认菜单。

支持两种模式（Web UI：**基础配置 → 服务配置 → Netboot 目录**，或侧边栏底部 **设置 → Netboot**）：

1. **显示默认 Profile 的引导项** — 仅展示标记为 default 的 Profile
2. **列出所有 Profile** — 将所有 Profile 作为菜单项

配置项：
- **菜单标题** — 默认 `PxeLab Boot Menu`
- **超时时间** — 0 = 不自动选择，>0 = 超时后自动选择默认条目
- **菜单条目** — 可添加多个引导项

---

## 自定义 iPXE 脚本

在 **设置弹窗 → Netboot → 自定义 iPXE 脚本** 中填写后，**完全替代**所有可视化配置：

<v-pre>

```
#!ipxe
dhcp || clear
echo Booting custom script for {{.MAC}}
chain {{.URL}}/boot/custom.ipxe || shell
```

</v-pre>

可用模板变量：
- `{`{`.URL}` — 服务器地址（如 `http://192.168.1.10:8080`）
- `{`{`.MAC}` — 客户端 MAC 地址
- `{`{`.NextServer}` — 服务器 IP（不含端口）
