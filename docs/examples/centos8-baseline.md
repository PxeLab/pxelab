# CentOS 8 自动化安装 + PxeLab 初始化基线（端到端示例）

> 目标：PXE 装机装完 **CentOS 8** 后，平台自动拉取并执行这台机器绑定的初始化基线（如系统更新、SSH 加固），全程无需在每份应答模板里手写分发逻辑。

## 涉及的功能

| 模块 | 作用 |
|---|---|
| 脚本库（初始化脚本 → 脚本库） | 单个可执行脚本（shell/bat/powershell），内容的唯一编辑地 |
| 初始化脚本 / 脚本集（Baseline） | 有序组合脚本库脚本，可挂到 Profile 或主机 |
| 主机（初始化脚本勾选区） | 每台主机可继承 Profile 的基线，并可额外勾选脚本集/单脚本 |
| 应答模板开关 `enable_baseline_pull` | 分发应答文件时自动注入“拉取并执行基线”钩子 |

## 链路

```
Profile(引导菜单) → CentOS 8 菜单 → kickstart 应答文件(勾选了自动下发)
                                              │ 平台按主机 MAC/SN 自动注入 %post
                                              ▼
                        GET /api/v1/baselines/pull.sh?mac=52:54:00:xx:xx:xx
                                              │ 返回该主机全部生效 shell 脚本(已渲染)
                                              ▼
                        逐个执行（无配置则 exit 0，不打断安装）
```

## 一、准备脚本（脚本库）

### `centos8-system-update`

```bash
#!/bin/bash
if ! command -v dnf >/dev/null 2>&1; then
  echo "[pxelab] dnf not found, skip update"
  exit 0
fi
dnf -y update
dnf clean all
```

### `centos8-sshd-harden`

```bash
#!/bin/bash
CFG=/etc/ssh/sshd_config
[ -f "$CFG" ] || { echo "[pxelab] no sshd_config"; exit 0; }
# 备份
cp "$CFG" "${CFG}.pxelab.bak"
# 示例：禁止 root 密码登录（请按需调整）
sed -i 's/^#\?PermitRootLogin .*/PermitRootLogin prohibit-password/' "$CFG"
# 示例：打开 PubkeyAuthentication（注释掉即默认 no 时不适用）
sed -i 's/^#\?PubkeyAuthentication .*/PubkeyAuthentication yes/' "$CFG"
if command -v systemctl >/dev/null 2>&1; then
  systemctl restart sshd || systemctl restart ssh || true
fi
```

## 二、建立脚本集（初始化脚本）

1. 初始化脚本 → 脚本集 → 新建 `centos8-security`
2. 添加脚本：从脚本库选择上面两个（顺序即执行顺序，可用上下键调整）
   - 1️⃣ `centos8-system-update`
   - 2️⃣ `centos8-sshd-harden`

## 三、应答模板（kickstart）

1. 应答模板 → 新建
2. 类型：`kickstart`
3. 勾选：**安装时自动拉取并执行该主机的初始化基线**
4. 内容粘贴下面正文，保存后点「预览」确认出现自动注入的 `%post` 段

```kickstart
# CentOS 8 自动化安装 + PxeLab 初始化基线
install
text
lang en_US.UTF-8
keyboard us
timezone Asia/Shanghai --utc
# 仓库按你的镜像源调整（示例 Stream 源）
url --url="http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"
network --bootproto=dhcp --device=link --activate
# rootpw 请替换为你自己生成的加密串（grub-crypt / openssl passwd -6）
rootpw --iscrypted $6$ROUNDS$REPLACE_WITH_YOUR_HASH
selinux --enforcing
firewall --enabled --service=ssh
services --enabled=sshd,chronyd
skipx
reboot

zerombr
clearpart --all --initlabel
autopart --type=lvm

%packages
@^minimal-environment
openssh-server
chrony
# 保证安装后 %post 有取件工具（平台钩子对 curl/wget 均有兜底）
curl
wget
%end

%post --log=/root/pxelab-post.log
echo 'PXE install done on host $(hostname)'
%end
```

## 四、预期：平台自动注入的内容

预览/分发时，平台会在模板末尾自动追加（MAC 为任务主机真实值，无需手填）：

```kickstart
%post --interpreter=/bin/bash
# PxeLab baseline pull (pxelab-baseline-pull)
set +e
if command -v curl >/dev/null 2>&1; then curl -fsSL 'http://PXELAB_IP:8080/api/v1/baselines/pull.sh?mac=52:54:00:aa:bb:cc' | bash
elif command -v wget >/dev/null 2>&1; then wget -qO- 'http://PXELAB_IP:8080/api/v1/baselines/pull.sh?mac=52:54:00:aa:bb:cc' | bash
fi
%end
```

- 身份：默认 `mac`；若配置 `global.identity_attr: sn` 则注入 `?sn=…`
- `pull.sh` 只包含该主机的 **shell** 类型脚本（Windows 用 `pull.ps1`）
- 无任何配置的主机返回空 → `exit 0`，安装不受影响
- 调试：装完查看 `/root/pxelab-post.log`

## 五、主机绑定

1. 主机管理 → 添加主机：填真实 MAC
2. “初始化脚本”区：若该主机挂的 Profile 已勾选脚本集会自动继承（只读展示）；也可在此直接追加脚本集/脚本
3. 让该主机走本模板（引导菜单 / 安装任务指向此 kickstart 模板）

## 排查清单

- 模板预览没出现 `%post` 注入段 → 确认已勾选开关且类型为 `kickstart`（其他类型注入点不同）
- 装完没执行脚本 → 看 `/root/pxelab-post.log`；确认主机已注册且 MAC 匹配、已勾选脚本集
- `curl`/`wget` 都不在 → 在 `%packages` 里显式加 `curl wget`（如上）
