# Netboot 自动化应答与个性化覆盖设计

## 概述

在现有 netboot catalog 基础上，增加三层能力：
1. **个性化覆盖** — 覆盖上游 catalog 的 URL、cmdline、应答注入参数
2. **应答模板管理** — 在网页上管理 kickstart/preseed/autounattend 等模板
3. **安装任务分配** — 为主机指定引导配置 + 应答模板，实现全自动安装

## 架构分层

```
上游 netboot.xyz git repo
        │ git sync (文件系统)
        ▼
data_dir/netboot/catalog/*.yaml   ◄── 只读，sync 完全覆盖
        │
        │ 加载时合并 DB overlay
        ▼
    DB netboot_overlays            ◄── 网页编辑，存储覆盖字段
        │
        │ iPXE 脚本生成时查安装任务
        ▼
    DB install_tasks               ◄── 关联主机 + 引导版本 + 应答模板
        │
        ▼
    iPXE 引导脚本 (kernel + cmdline + answer_param)
```

## DB Schema

### netboot_overlays

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| distro_name | TEXT UNIQUE | 对应 catalog 文件名（如 `ubuntu`） |
| enabled | BOOLEAN | 是否启用覆盖 |
| mirror | TEXT | 覆盖镜像源 URL |
| local_base | TEXT | 覆盖本地文件路径 |
| kernel_params | TEXT | 追加到所有版本的全局 kernel 参数 |
| version_overrides | JSON | 版本级覆盖数组 |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

**version_overrides JSON 结构：**

```json
[
  {
    "codename": "noble",
    "arch": "amd64",
    "enabled": true,
    "remote_kernel": "http://mirror.local/ubuntu/noble/amd64/linux",
    "remote_initrd": "http://mirror.local/ubuntu/noble/amd64/initrd.gz",
    "cmdline": "net.ifnames=0 biosdevname=0",
    "answer_param": "autoinstall ds=nocloud-net;s={{.AnswerURL}}",
    "answer_type": "subiquity"
  }
]
```

**answer_param 注入格式示例：**

| 系统 | answer_param |
|------|-------------|
| Ubuntu Subiquity | `autoinstall ds=nocloud-net;s={{.AnswerURL}}` |
| Ubuntu/Debian Preseed | `preseed/url={{.AnswerURL}}` |
| RHEL/CentOS/Rocky/Alma | `inst.ks={{.AnswerURL}}` |
| Fedora | `inst.ks={{.AnswerURL}}` |
| SUSE | `autoyast={{.AnswerURL}}` |
| ESXi | `ks={{.AnswerURL}}` |
| Windows | 特殊处理（见下文） |

### answer_templates

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| name | TEXT | 模板名称 |
| description | TEXT | 说明 |
| type | TEXT | kickstart / preseed / subiquity / autoyast / autounattend |
| content | TEXT | 模板内容 |
| variables | JSON | 模板中使用的变量列表（用于 UI 提示） |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

**模板变量支持：**

所有模板支持运行时变量替换：

| 变量 | 说明 |
|------|------|
| `{{.HostName}}` | 主机名 |
| `{{.HostIP}}` | 主机 IP |
| `{{.HostMAC}}` | 主机 MAC |
| `{{.HostCIDR}}` | 子网 CIDR |
| `{{.Gateway}}` | 网关 |
| `{{.DNSServers}}` | DNS 服务器 |
| `{{.Disk}}` | 安装磁盘（默认 `/dev/sda`） |
| `{{.KeyboardLayout}}` | 键盘布局 |

Windows autounattend 额外变量：

| 变量 | 说明 |
|------|------|
| `{{.ProductKey}}` | Windows 产品密钥 |
| `{{.ComputerName}}` | 计算机名 |
| `{{.JoinDomain}}` | 加入的域 |
| `{{.DomainOU}}` | 域 OU |
| `{{.AdminPassword}}` | 管理员密码 |
| `{{.TimeZone}}` | 时区 |

### install_tasks

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT PK（如 `task_<uuid>`） | |
| host_id | TEXT FK → hosts.id | 目标主机 |
| distro_name | TEXT | catalog distro 名称 |
| version_codename | TEXT | 版本 codename |
| answer_template_id | INTEGER FK → answer_templates.id | 使用的应答模板（可选） |
| extra_cmdline | TEXT | 额外 kernel 参数 |
| status | TEXT | pending / installing / done / failed |
| error_msg | TEXT | 失败原因 |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

## 应答注入机制

### Linux/ESXi 类

iPXE 脚本生成流程：

1. 加载 catalog YAML
2. 从 DB 加载 overlay 合并（URL、cmdline、answer_param）
3. 对于每个要渲染的版本，检查 DB 中该主机是否有 `install_tasks`
4. 如果有任务：
   a. 渲染 answer_template → 存储并通过 `/api/v1/netboot/answer/<task_id>` 提供
   b. 在 kernel 行追加 cmdline，并按照 `answer_param` 格式注入 `{{.AnswerURL}}` 替换后的字符串
5. 如果没有任务：按原有逻辑生成标准引导行

**enhanced `GenerateBootLine` 逻辑：**

```go
func GenerateBootLine(v *Version, serverAddr, bootPrefix, kernelParams string, task *InstallTask) string {
    // 基础 URL 构建（与现有逻辑相同）
    
    // 如果存在安装任务
    if task != nil {
        answerParam := v.AnswerParam  // 从 overlay 获取
        if answerParam != "" {
            answerURL := fmt.Sprintf("http://%s/api/v1/netboot/answer/%s", serverAddr, task.ID)
            answerParam = strings.ReplaceAll(answerParam, "{{.AnswerURL}}", answerURL)
            fullCmdline = strings.TrimSpace(fullCmdline + " " + answerParam)
        }
    }
    
    // 生成 boot 行
}
```

### Windows 类（特殊处理）

Windows 通过 wimboot 加载，autounattend.xml 需要作为 initrd 注入：

```ipxe
kernel http://server/boot/wimboot
initrd -n bootmgr          http://server/path/bootmgr          bootmgr
initrd -n bootmgr.efi      http://server/path/bootmgr.efi      bootmgr.efi
initrd -n bcd              http://server/path/boot/bcd          bcd
initrd -n boot.sdi         http://server/path/boot/boot.sdi     boot.sdi
initrd -n boot.wim         http://server/path/sources/boot.wim  boot.wim
initrd -n autounattend.xml http://server/api/v1/netboot/answer/<task_id> autounattend.xml
boot
```

在 Version overlay 中标识：
```json
{
  "answer_param_wimboot": true
}
```

## API 端点

### 覆盖管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/netboot/overlays` | 列出所有 overlay |
| GET | `/api/v1/netboot/overlays/{distro}` | 获取单个 overlay |
| PUT | `/api/v1/netboot/overlays/{distro}` | 创建/更新 overlay |
| DELETE | `/api/v1/netboot/overlays/{distro}` | 删除 overlay |

### 应答模板管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/netboot/answer-templates` | 列出模板 |
| POST | `/api/v1/netboot/answer-templates` | 创建模板 |
| GET | `/api/v1/netboot/answer-templates/{id}` | 获取模板 |
| PUT | `/api/v1/netboot/answer-templates/{id}` | 更新模板 |
| DELETE | `/api/v1/netboot/answer-templates/{id}` | 删除模板 |

### 安装任务管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/netboot/tasks` | 列出任务 |
| POST | `/api/v1/netboot/tasks` | 创建任务 |
| GET | `/api/v1/netboot/tasks/{id}` | 获取任务 |
| PUT | `/api/v1/netboot/tasks/{id}` | 更新任务 |
| DELETE | `/api/v1/netboot/tasks/{id}` | 删除任务 |

### PXE 运行时（无认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/netboot/task/by-mac/{mac}` | 查询主机安装任务配置（返回 answer_param 和 extra_cmdline） |
| GET | `/api/v1/netboot/answer/{task_id}` | 获取渲染后的应答文件内容 |

## 前端页面

### Overlay 编辑页

从现有 NetbootCatalog 页面扩展，每个 distro 详情页增加"覆盖配置"标签：
- 编辑 mirror / kernel_params
- 启用/禁用版本，覆盖 URL
- 配置 cmdline 和 answer_param
- 选择 answer 模板类型

### 应答模板管理页

新增页面 `/netboot/answer-templates`：
- 模板列表（名称、类型、更新时间）
- 内置编辑器（CodeMirror 或 textarea + 语法提示）
- 模板变量提示面板

### 主机安装任务

在主机详情页 `/hosts/{id}` 增加"安装任务"标签：
- 选择 distro + version（从 catalog + overlay 合并后的列表）
- 选择应答模板
- 填写额外 cmdline
- 查看任务状态和日志

## 启动流程（完整链路）

```
1. 管理员配置 overlay（网页）：
   - 修改 ubuntu noble 的镜像 URL
   - 设置 answer_param = "autoinstall ds=nocloud-net;s={{.AnswerURL}}"

2. 管理员创建应答模板（网页）：
   - 写 user-data 模板，保存为 subiquity 类型

3. 管理员分配任务（网页）：
   - 选择主机 → 选择 Ubuntu Noble → 选择应答模板 → 创建

4. 主机 PXE 启动：
   - DHCP → iPXE → 获取 http://server/boot/ipxe/script?mac=...
   - iPXE 脚本包含 Ubuntu Noble 菜单项

5. 用户选择 Ubuntu Noble（或自动选）：
   - PxeGo 检测该主机有 install_task
   - 渲染应答模板 → 存储到临时路径
   - kernel 行注入 cmdline: "autoinstall ds=nocloud-net;s=http://server/api/v1/netboot/answer/task_xxx"
   - 启动安装程序

6. 安装程序自动请求应答文件：
   - GET /api/v1/netboot/answer/task_xxx
   - 返回渲染后的 user-data
   - 全自动安装
```

## 同步与合并规则

1. **上游 sync** 直接写文件系统 `catalog/*.yaml`，完全覆盖
2. **加载时**：读取 catalog YAML → 查询 DB overlay → 按 `distro_name` 匹配
3. **合并策略**：overlay 中非空字段覆盖 catalog（深层合并 version_overrides 以 codename+arch 为 key）
4. **不存在的 distro** 可以在 overlay 中新建（需要填写完整引导信息），用于添加上游没有的自定义系统

## 安全考虑

- `/api/v1/netboot/task/by-mac/{mac}` 和 `/api/v1/netboot/answer/{task_id}` 需要在 `isPublicPath` 中白名单，因为 PXE 客户端不携带认证头
- 应答文件可能包含密码等敏感信息 → 渲染为一次性访问或限制只能从 PXE 客户端 IP 访问
