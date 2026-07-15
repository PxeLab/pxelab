# PxeLab 自动化运维编排引擎 — 设计规格书 v2

## 0. 术语说明

| 术语 | 说明 |
|------|------|
| **JobPlan** | 运维计划模板，描述一组有序的 Stage。类型：os-install / firmware / diagnostic / config / custom |
| **JobTask** | 运维任务实例，JobPlan 的一次执行。一个 Host 同一时间只有一个活跃 JobTask |
| **Stage** | JobPlan 内的一个执行阶段，是编排的最小单元 |
| **BootOS** | PxeLab 配发的微型 Linux 运行环境（initrd-based），内含 Agent |
| **Agent** | BootOS 内运行的客户端程序，负责执行 Stage 指令并与 Server 通信 |
| **DiscoveredDevice** | 新发现但尚未纳入管理的设备 |

---

## 1. 整体架构与流程

### 1.1 核心原理

**每次 PXE 启动 = 一次轮询**。客户端问"我该做什么"，Server 查该 MAC 的状态决定返回什么。

```
PXE 启动 /boot/ipxe/script?mac=xx
  │
  ▼
Server:
  ├─ 有活跃 JobTask → 查当前 Stage
  │   ├─ pre-install   → 返回 BootOS 引导脚本
  │   ├─ install       → 返回 OS Installer 引导脚本
  │   ├─ firmware      → 返回 BootOS 引导脚本
  │   ├─ post-install  → 返回 localboot（新系统引导）
  │   └─ DONE          → 标记完成,返回 localboot
  │
  ├─ 无 JobTask → 查 MAC 是否已注册为 Host
  │   ├─ 是 → 查 MatchPolicy
  │   │   ├─ 匹配 → 自动创建 JobTask，进入安装流程
  │   │   └─ 不匹配 → 返回传统 Profile 菜单
  │   └─ 否 → 返回 BootOS 发现脚本 (MODE=discover)
  │
  └─ Agent 回调路径
      POST /api/v1/jobs/by-mac/{mac}
      POST /api/v1/jobs/{id}/inventory
      POST /api/v1/jobs/{id}/stage/{order}/heartbeat
      POST /api/v1/jobs/{id}/stage/{order}/done
      POST /api/v1/jobs/{id}/stage/{order}/fail
      POST /api/v1/jobs/{id}/log
```

### 1.2 完整生命周期

```
裸机上架
  │ PXE 首次启动
  ▼
┌─────────────────────────────────┐
│ 发现模式 (Discovery)            │
│ Device → DiscoveredDevice       │
│ 等待管理员处理                  │
└──────────────┬──────────────────┘
               │ 管理员 adopt
               ▼
┌─────────────────────────────────┐
│ 注册为 Host + 创建 JobTask      │
│ 触发 WOL / 人工重启             │
└──────────────┬──────────────────┘
               │ PXE 再次启动，进入执行模式
               ▼
┌──────────────────────────────────┐
│ Stage 1: pre-install / firmware  │
│ BootOS + Agent 执行              │
│ ├─ 配置 RAID                     │
│ ├─ 更新固件                      │
│ ├─ 硬件检测                      │
│ └─ POST /done → reboot           │
└──────────────┬───────────────────┘
               │ 重启 → PXE 第三次启动
               ▼
┌──────────────────────────────────┐
│ Stage 2: install                 │
│ OS Installer (kernel+initrd)     │
│ 自动应答文件 (含注入的进度回调)   │
│ └─ 安装完成 → reboot             │
└──────────────┬───────────────────┘
               │ 重启 → 新系统首次启动
               ▼
┌──────────────────────────────────┐
│ Stage 3: post-install            │
│ Agent / cloud-init / %post 脚本  │
│ └─ 配置网络 / 安装软件 / 加域    │
└──────────────┬───────────────────┘
               │ 完成
               ▼
┌──────────────────────────────────┐
│ DONE                             │
│ JobTask 标记完成                 │
└──────────────────────────────────┘
```

---

## 2. 数据模型

### 2.1 JobPlan — 运维计划

```yaml
job_plans:
  - id: "rhel9-dell-prod"
    name: "RHEL 9 - Dell 生产部署"
    type: os-install               # os-install | firmware | diagnostic | config | custom
    description: "Dell R740 装 RHEL 9 + 生产软件栈"
    default_boot_template_id: "bootos-v1"
    variables:
      timezone: "Asia/Shanghai"
      dns: "10.0.0.1"
    estimated_duration: 45          # 预估分钟，用于 Stage 2 进度插值
    stages:
      - order: 1
        name: "RAID 配置"
        phase: pre-install
        boot_template_id: "bootos-v1"
        raid_config_id: "r740-ssd-raid1"
        script_ids: ["inventory-report"]
        timeout: 20

      - order: 2
        name: "安装 RHEL 9"
        phase: install
        boot_template_id: "rhel9-installer"
        answer_template_id: "rhel9-server-ks"
        script_ids: ["configure-network-bond"]
        timeout: 60
        on_failure: abort

      - order: 3
        name: "首次启动配置"
        phase: post-install
        script_ids: ["install-docker", "join-domain"]
        timeout: 30
        on_failure: continue
        when: "{{.Vars.install_docker}}"

  - id: "fw-update-dell-r740"
    name: "Dell R740 固件更新"
    type: firmware
    description: "更新 BIOS/BMC/RAID 固件到指定版本"
    default_boot_template_id: "bootos-v1"
    stages:
      - order: 1
        name: "更新 BIOS"
        phase: firmware
        script_ids: ["update-dell-bios"]
        firmware_files:
          - url: "http://repo/dell/BIOS_R740_2.15.exe"
            dest: "/fw/bios.exe"
            sha256: ""
        timeout: 15

      - order: 2
        name: "更新 BMC"
        phase: firmware
        script_ids: ["update-dell-bmc"]
        firmware_files:
          - url: "http://repo/dell/BMC_R740_2.85.exe"
            dest: "/fw/bmc.exe"
        timeout: 15

      - order: 3
        name: "更新 RAID 固件"
        phase: firmware
        script_ids: ["update-dell-perc"]
        firmware_files:
          - url: "http://repo/dell/PERC_R740_50.6.bin"
            dest: "/fw/perc.bin"
        timeout: 10

  - id: "mem-diag"
    name: "内存诊断"
    type: diagnostic
    default_boot_template_id: "bootos-v1"
    stages:
      - order: 1
        name: "Memtest86+"
        phase: custom
        boot_template_id: "memtest-boot"   # 直接引导 memtest，非 BootOS
        timeout: 120
```

**JobPlan 字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识 |
| name | string | 可读名称 |
| type | enum | os-install / firmware / diagnostic / config / custom |
| description | string | 描述 |
| default_boot_template_id | string | 默认的 BootOS 引导模板 |
| variables | map | 计划级默认变量 |
| estimated_duration | int | 预估总时长（分钟），用于进度插值 |
| stages | []Stage | 阶段列表（按 order 排序顺序执行） |

### 2.2 Stage — 阶段

```yaml
stages:
  - order: 1
    name: "RAID 配置"
    phase: pre-install          # pre-install | install | post-install | firmware | custom
    boot_template_id: "bootos-v1"
    raid_config_id: "r740-ssd-raid1"
    answer_template_id: ""      # 仅 install phase 时使用
    script_ids: ["inventory-report"]
    firmware_files: []          # 仅 firmware phase 时使用
    timeout: 30                 # 分钟
    on_failure: abort           # abort | continue | retry(N)
    when: "{{.Vars.has_raid}}"  # 条件表达式，false 则跳过
```

| 字段 | 类型 | 说明 |
|------|------|------|
| order | int | 执行顺序 |
| name | string | 可读名称 |
| phase | enum | 所属阶段类型 |
| boot_template_id | string | 本阶段引导环境（空=继承 JobPlan 默认） |
| raid_config_id | string | RAID 配置（仅 pre-install）|
| answer_template_id | string | 应答模板（仅 install）|
| script_ids | []string | 本阶段执行的脚本 |
| firmware_files | []FirmwareFile | 固件文件（仅 firmware）|
| timeout | int | 超时分钟数，0=不限 |
| on_failure | string | 失败策略 |
| when | string | 条件表达式 |

**FirmwareFile**：

| 字段 | 类型 | 说明 |
|------|------|------|
| url | string | 固件文件下载 URL |
| dest | string | Agent 内目标路径 |
| sha256 | string | 校验和（可选）|

### 2.3 BootTemplate — 引导模板

```yaml
boot_templates:
  - id: "bootos-v1"
    name: "PxeLab BootOS v1"
    boot_type: "kernel"
    kernel: "http://{{.Server}}/boot/bootos/vmlinuz"
    initrd: "http://{{.Server}}/boot/bootos/initrd.img"
    cmdline: |
      PXELAB_SERVER={{.Server}}
      PXELAB_TASK_ID={{.TaskID}}
      PXELAB_TOKEN={{.Token}}
      PXELAB_MODE={{.Mode}}          # discover | execute
      console=tty0 console=ttyS0,115200
    arch: "amd64"

  - id: "rhel9-installer"
    name: "RHEL 9 Installer"
    boot_type: "kernel"
    kernel: "http://mirror.internal/el9/vmlinuz"
    initrd: "http://mirror.internal/el9/initrd.img"
    cmdline: "inst.repo=http://mirror.internal/el9 inst.ks={{.AnswerURL}} console=ttyS0,115200"
    arch: "amd64"
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识 |
| name | string | 可读名称 |
| boot_type | enum | kernel / memdisk / wimboot / sanboot |
| kernel | string | 内核 URL（支持模板变量）|
| initrd | string | Initrd URL（支持模板变量）|
| cmdline | string | 内核参数（支持模板变量）|
| arch | string | 架构过滤，空=全匹配 |

**模板变量**：

| 变量 | 说明 |
|------|------|
| {{.Server}} | PxeLab HTTP 地址 |
| {{.TaskID}} | 任务 ID |
| {{.Token}} | 回调认证 Token |
| {{.Mode}} | discover / execute |
| {{.MAC}} | 客户端 MAC |
| {{.AnswerURL}} | 应答文件下载 URL |

### 2.4 RaidConfig — RAID 配置

```yaml
raid_configs:
  - id: "r740-ssd-raid1"
    name: "Dell R740 SSD RAID1 + HDD RAID5"
    controller_match:
      vendor: "Dell"
      model: "PowerEdge R740"
    arrays:
      - raid: 1
        drives:
          selector: "first:2,media:ssd,size:>=480"
        name: "OS"
      - raid: 5
        drives:
          selector: "remaining"
        name: "DATA"
    hot_spare:
      drives:
        selector: "first:1,media:hdd"
    fallback:
      commands:
        - "storcli64 /c0 add vd type=raid1 name=OS drives=0:0,0:1"
        - "storcli64 /c0 add vd type=raid5 name=DATA drives=0:2-0:5"
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识 |
| name | string | 可读名称 |
| controller_match | map | 控制器匹配条件（用于自动选择 Translator）|
| arrays | []ArraySpec | RAID 数组声明 |
| hot_spare | DriveSelector | 热备盘 |
| init_command | string | 可选：清空配置的前置命令 |
| fallback | RawCommands | 兜底 Raw CLI |
| boot_template_id | string | 可选：自定义引导环境 |

**DriveSelector DSL 语法**：

```
selector = condition ("," condition)*

condition = "first:" N            → 前 N 块盘
          | "last:" N             → 后 N 块盘
          | "media:" ("ssd"|"hdd"|"nvme")
          | "size:" (">="|"<="|"==") NUM ("G"|"T")
          | "size:" NUM "G" "-" NUM "G"
          | "model:" glob
          | "slot:" slot ("," slot)*
          | "remaining"           → 剩余所有盘
          | "all"                 → 所有盘

示例:
  "first:2,media:ssd,size:>=480"    → 前 2 块 ≥480G SSD
  "remaining"                        → 剩余盘
  "slot:0:0,0:1"                     → 指定槽位
```

**RAID 安全规则**：
1. 执行前预览命令列表，用户确认
2. 执行前备份当前配置到 Server
3. 幂等检查：目标盘是否已有 RAID
4. 生产环境二次确认

### 2.5 ScriptTemplate — 脚本模板

```yaml
script_templates:
  - id: "configure-network-bond"
    name: "网卡绑定"
    phase: post-install
    os: "el"
    content: |
      #!/bin/bash
      nmcli con add type bond ifname bond0 mode 802.3ad
      nmcli con add type bond-slave ifname {{.Vars.nic1}} master bond0
      nmcli con add type bond-slave ifname {{.Vars.nic2}} master bond0
      nmcli con mod bond0 ipv4.addresses {{.Vars.ip}}/{{.Vars.netmask}}
      nmcli con up bond0
    variables:
      nic1: "eno1"
      nic2: "eno2"
      ip: ""
      netmask: "24"
      gateway: ""

  - id: "update-dell-bios"
    name: "更新 Dell BIOS"
    phase: firmware
    os: "el"
    content: |
      #!/bin/bash
      # {{.FirmwareFiles.bios_bin.dest}} 由 firmware_files 自动注入
      /fw/bios.exe --silent --reboot=false
      if [ $? -ne 0 ]; then
        echo "BIOS 更新失败"
        exit 1
      fi
      echo "BIOS 更新成功，固件版本: $(dmidecode -s bios-version)"
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识 |
| name | string | 可读名称 |
| phase | string | 适用阶段，用于筛选 |
| os | string | 目标 OS 类型（el/debian/suse/windows）|
| content | string | 脚本内容（Go template）|
| variables | map | 变量声明及默认值 |
| builtin | bool | 是否内置（Agent 自带，无需下发）|

### 2.6 MatchPolicy — 匹配策略

```yaml
match_policies:
  - id: "pol-dell-r740-rhel"
    name: "Dell R740 → RHEL 9"
    priority: 100
    conditions:
      vendor: "Dell"
      model: "PowerEdge R740"
      arch: "amd64"
      labels: ["rhel9"]
    job_plan_id: "rhel9-dell-prod"
    vars:
      has_raid: "true"
      install_docker: "true"

  - id: "pol-dell-r740-fw"
    name: "Dell R740 → 固件更新"
    priority: 90
    conditions:
      vendor: "Dell"
      model: "PowerEdge R740"
      labels: ["fw-update"]
    job_plan_id: "fw-update-dell-r740"

  - id: "pol-fallback-x64"
    name: "通用 x86_64 兜底"
    priority: 1
    conditions:
      arch: "amd64"
    job_plan_id: "minimal-rocky9"
    vars: {}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识 |
| name | string | 可读名称 |
| priority | int | 优先级，越大越先匹配 |
| conditions | MatchConditions | 匹配条件 |
| job_plan_id | string | 匹配成功后关联的 JobPlan |
| vars | map | 匹配后注入的变量 |

**MatchConditions**：

| 字段 | 类型 | 说明 |
|------|------|------|
| vendor | string | 厂商（来源 BMC FRU / DMI）|
| model | string | 型号，支持 glob |
| arch | string | 架构（来源 DHCP Option 93）|
| labels | []string | Host 标签 |
| mac_prefix | string | MAC 前缀 |

### 2.7 JobTask — 任务实例

```go
type JobTask struct {
    ID              string
    JobPlanID       string
    HostID          string
    Type            string       // os-install | firmware | diagnostic | config | custom
    Status          string       // pending | running | done | failed | cancelled
    CurrentStage    int          // 当前 Stage 序号
    StageStatus     JSON         // map[int]string: pending/running/done/skip/fail
    StageProgress   JSON         // map[int]int: 每个 Stage 的进度 0-100
    Token           string       // Agent 回调认证
    Vars            JSON         // 合并后的运行时变量
    SkipStages      []int        // 要跳过的 Stage
    StartStage      int          // 起始 Stage
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

### 2.8 DiscoveredDevice — 发现设备

```go
type DiscoveredDevice struct {
    ID          string
    MAC         string
    IP          string
    Vendor      string
    Model       string
    Serial      string
    Arch        string
    Inventory   JSON           // 完整的硬件探测数据
    Status      string         // pending | adopted | ignored
    FirstSeen   time.Time
    LastSeen    time.Time
}
```

**发现流程**：
1. 未注册 MAC 的机器 PXE 启动，Server 返回 BootOS（`MODE=discover`）
2. BootOS 内 Agent 采集全部硬件信息
3. Agent `POST /api/v1/discovery` 上报
4. Server 创建 DiscoveredDevice，返回"等待管理员处理"
5. Agent 进入低功耗循环轮询（或直接关机等待）
6. 管理员在 UI "发现设备"页看到该设备，查看硬件详情
7. 管理员选择操作：
   - **adopt**: 创建正式 Host，可选择关联 JobPlan、设置变量
   - **ignore**: 设备忽略，不再重复发现
   - **批量 adopt**: 勾选多台 + 批量设置变量

### 2.9 现有模型扩展

**Host**：

```go
type Host struct {
    // ... 现有字段
    InstallVars  JSON     `json:"install_vars"`   // map[string]string
    Labels       JSON     `json:"labels"`          // []string
    MatchPolicyID *string `json:"match_policy_id"` // 强制指定策略
    ActiveJobID  *string  `json:"active_job_id"`   // 当前活跃 JobTask
}
```

---

## 3. 状态机

```
                    ┌──────────┐
                    │  PENDING │  → 可选 skip_stages
                    └────┬─────┘
                         │ PXE 启动 → 轮询
                    ┌────▼─────┐
                    │  RUNNING │
                    └────┬─────┘
                         │ 进入当前 Stage
              ┌──────────┼───────────┬──────────┐
              ▼          ▼           ▼          ▼
       ┌──────────┐ ┌───────┐ ┌─────────┐ ┌───────┐
       │PRE_INST  │ │INSTALL│ │FIRMWARE │ │ CUSTOM│
       │(BootOS)  │ │(Installer)│(BootOS)│ │(BootOS)│
       └────┬─────┘ └───┬───┘ └────┬────┘ └───┬───┘
            │           │          │          │
            ▼           ▼          ▼          ▼
      Agent  回拨   %pre/%post   Agent 回拨  Agent 回拨
      POST done   自动回拨      POST done    POST done
            │           │          │          │
            └───────┬───┘──────────┘──────────┘
                    ▼
              ┌──────────┐
              │  NEXT    │ → 还有下个 Stage？→ RUNNING
              └────┬─────┘
                   │ 无下个 Stage
              ┌────▼─────┐
              │   DONE   │
              └──────────┘

所有阶段均可进入 FAILED → RETRYING (on_failure=retry)
管理员可:
  FAILED → RETRY (人工修复后重试)
  RUNNING → CANCEL (取消)
  DONE → 重新创建 JobTask
```

---

## 4. Agent ↔ Server API 契约

### 4.1 基础信息

- **协议**: HTTP REST
- **Base URL**: 从 cmdline `PXELAB_SERVER` 获取
- **认证**: `X-PXELAB-Token` Header 或 `?token=` Query 参数
- **Content-Type**: `application/json`

### 4.2 Agent 端点

#### GET /api/v1/discovery

仅发现模式使用。Agent 向 Server 报告设备已就绪，等待任务。

**Response 200**：

```json
{
  "action": "wait",            // wait | adopt | shutdown
  "message": "设备已注册，等待管理员处理"
}
```

#### POST /api/v1/discovery/inventory

Agent 上报完整的硬件探测数据。

**Request**：

```json
{
  "mac": "00:1A:64:01:01:01",
  "vendor": "Dell",
  "model": "PowerEdge R740",
  "serial": "ABC123",
  "arch": "x86_64",
  "bios_version": "2.12.1",
  "disks": [
    {"slot": "0:0", "model": "INTEL SSD D3-S4510", "size_gb": 480, "media": "ssd", "status": "online"},
    {"slot": "0:1", "model": "INTEL SSD D3-S4510", "size_gb": 480, "media": "ssd", "status": "online"}
  ],
  "raid_controllers": [
    {"model": "PERC H740P", "slot": 0, "driver": "megaraid_sas", "firmware": "50.5.1"}
  ],
  "nics": [
    {"name": "eno1", "mac": "00:1A:64:01:01:01", "speed": 25000}
  ],
  "memory_gb": 192,
  "cpu": {"model": "Intel Xeon Gold 6248R", "cores": 48}
}
```

**Response 200**：

```json
{
  "device_id": "dev-xxx",
  "action": "wait"
}
```

#### GET /api/v1/jobs/by-mac/{mac}

Agent 启动后拉取任务信息。执行模式和发现模式共用。

**Response 200（有活跃任务）**：

```json
{
  "job_id": "job-xxx",
  "type": "os-install",
  "status": "running",
  "current_stage": 1,
  "stage": {
    "order": 1,
    "name": "RAID 配置",
    "phase": "pre-install",
    "scripts": [
      {"id": "inventory-report", "content": "#!/bin/sh\n...", "variables": {}}
    ],
    "raid_config": {
      "arrays": [{"raid": 1, "drives": {"selector": "first:2,media:ssd"}}],
      "fallback": {"commands": ["storcli64 ..."]}
    },
    "variables": {"has_raid": "true"}
  }
}
```

**Response 200（无活跃任务）**：

```json
{
  "job_id": "",
  "action": "discover"   // 进入发现模式
}
```

#### POST /api/v1/jobs/{id}/stage/{order}/heartbeat

Agent 定期上报进度（推荐 30-60s 间隔）。

**Request**：

```json
{
  "progress": 45,
  "message": "创建 RAID 虚拟磁盘 1/2...",
  "logs": [
    {"level": "info", "message": "storcli64 /c0 add vd type=raid1 drives=0:0,0:1"}
  ]
}
```

**Response 200**：

```json
{
  "action": "continue"
}
```

**特殊 Response**：

```json
{"action": "abort", "reason": "管理员取消任务"}
{"action": "skip", "reason": "超时跳过，进入下一 Stage"}
```

#### POST /api/v1/jobs/{id}/stage/{order}/done

Agent 报告阶段完成。

**Request**：

```json
{
  "result": "ok",
  "summary": "RAID 配置完成",
  "details": [
    {"controller": 0, "vd": 0, "raid": 1, "size": "447GB"}
  ],
  "raid_config_snapshot": "..."
}
```

**Response 200**：

```json
{
  "next_action": "reboot",
  "message": "Stage 1 完成"
}
```

#### POST /api/v1/jobs/{id}/stage/{order}/fail

**Request**：

```json
{
  "error": "RAID 创建失败：无足够硬盘",
  "code": "RAID_INSUFFICIENT_DISKS",
  "details": "至少需要 2 块 SSD，发现 1 块"
}
```

**Response 200**：

```json
{
  "action": "abort",
  "message": "任务中止"
}
```

#### POST /api/v1/jobs/{id}/log

实时日志。

**Request**：

```json
{
  "lines": [
    {"level": "info", "message": "探测 RAID 控制器..."},
    {"level": "warn", "message": "固件版本过旧"}
  ]
}
```

---

## 5. 进度上报策略（分阶段）

| 阶段 | 粒度 | 实现方式 |
|------|------|----------|
| **pre-install** (BootOS) | 细粒度 0-100% | Agent 自行按步骤上报 |
| **firmware** (BootOS) | 细粒度 0-100% | Agent 自行按步骤上报 |
| **custom** (BootOS) | 细粒度 0-100% | Agent 自行按步骤上报 |
| **install** (OS Installer) | 粗粒度里程碑 | 自动注入回调 + 时间插值 |
| **post-install** | 细粒度 0-100% | Agent / cloud-init / 内嵌脚本上报 |

### 5.1 安装阶段（Stage 2）进度方案

```
创建 JobTask 时，Server 记录 start_time + estimated_duration
前端展示: "安装中 (预计剩余 XX 分钟)"

里程碑 1: %pre hook (Server 收到回调)
  → progress: 5   "安装器已启动"

[安装器静默运行]
  → Server 根据 start_time + estimated_duration 线性插值
  → 5% ~ 65% 之间自动推进
  (e.g. 已运行 10/30 分钟 → progress: 5 + (65-5) * (10/30) = 25%)

里程碑 2: %post hook (Server 收到回调)
  → progress: 70  "安装完成,执行配置脚本"
  → 停止插值

里程碑 3+: %post 内嵌脚本步骤 (各步完成后 curl 回拨)
  → progress: 75 / 85 / 95 / 100

重启/完成
  → POST /stage/2/done 或 Server 检测到超时自动推进
```

**自动注入机制**：

创建 JobTask 时，PxeLab 自动在 AnswerTemplate 的适当位置插入回调命令：

```
# kickstart %pre 自动注入
%pre --interpreter=/bin/bash
curl -s -X POST "http://{{.Server}}/api/v1/jobs/{{.TaskID}}/stage/2/heartbeat" \
  -H "X-PXELAB-Token: {{.Token}}" \
  -d '{"progress":5,"message":"安装器已启动"}'
%end

# kickstart %post 自动注入
%post --interpreter=/bin/bash
curl -s -X POST "http://{{.Server}}/api/v1/jobs/{{.TaskID}}/stage/2/heartbeat" \
  -H "X-PXELAB-Token: {{.Token}}" \
  -d '{"progress":70,"message":"安装完成，执行配置脚本"}'
# [原 %post 内容...]
curl -s -X POST "http://{{.Server}}/api/v1/jobs/{{.TaskID}}/stage/2/done" \
  -H "X-PXELAB-Token: {{.Token}}" \
  -d '{"result":"ok","summary":"OS 安装完成"}'
%end
```

用户不需要手动修改 answer file。

---

## 6. 变量体系

### 6.1 合并顺序

```
层级 1: 系统内置 (自动注入)
  {{.TaskID}}, {{.HostMAC}}, {{.Server}}, {{.AnswerURL}}

层级 2: JobPlan.variables
  {{.Vars.timezone}}, {{.Vars.dns}}

层级 3: MatchPolicy.vars
  {{.Vars.has_raid}}

层级 4: Host.InstallVars
  {{.Vars.hostname}}, {{.Vars.ip}}, {{.Vars.nic1}}

层级 5: JobTask 创建时传入
  {{.Vars.custom_param}}
```

后覆盖前。

### 6.2 使用场景

| 位置 | 引用方式 |
|------|---------|
| BootTemplate.cmdline | `{{.Server}}`, `{{.TaskID}}`, `{{.Token}}` |
| AnswerTemplate | `{{.Vars.hostname}}`, `{{.Vars.ip}}` |
| ScriptTemplate | `{{.Vars.nic1}}`, `{{.Vars.ip}}` |
| Stage.when | `{{.Vars.has_raid}}` |
| RaidConfig fallback | `{{.Vars.disk_slot1}}` |

### 6.3 DSL 函数

| 函数 | 用途 |
|------|------|
| `default "val" .Var` | 默认值 |
| `escape` | Shell 转义 |
| `bool .Var` | 布尔化（when 条件用）|

---

## 7. 发现流程详解

```
                     ┌────────────────────────┐
                     │ 设备上架，插电连网      │
                     │ PXE 启动                │
                     └──────────┬─────────────┘
                                ▼
                     ┌────────────────────────┐
                     │ DHCP: Server 分配 IP   │
                     │ TFTP: 下载 ipxe.efi    │
                     │ HTTP: /boot/ipxe/      │
                     │       script?mac=xx    │
                     └──────────┬─────────────┘
                                ▼
                     ┌────────────────────────┐
                     │ Server: MAC 未注册     │
                     │ → 返回 BootOS 发现脚本 │
                     │ kernel=bootos-vmlinuz  │
                     │ initrd=bootos-initrd   │
                     │ cmdline:               │
                     │   PXELAB_MODE=discover  │
                     └──────────┬─────────────┘
                                ▼
                     ┌────────────────────────┐
                     │ BootOS 启动            │
                     │ Agent 启动:            │
                     │ 1. 解析 cmdline        │
                     │ 2. 采集硬件信息        │
                     │    - DMI (dmidecode)   │
                     │    - 磁盘拓扑 (lsblk)  │
                     │    - RAID 控制器       │
                     │    - 网卡信息          │
                     │    - 内存/CPU          │
                     │ 3. POST /discovery/    │
                     │       inventory        │
                     └──────────┬─────────────┘
                                ▼
                     ┌────────────────────────┐
                     │ 等待管理员处理         │
                     │ Agent 每 30s 轮询      │
                     │ GET /api/v1/discovery  │
                     └──────────┬─────────────┘
                                │
                    ┌───────────┴───────────┐
                    │ 管理员在 UI 操作       │
                    │ 发现设备页:            │
                    │ 查看 20 台新设备       │
                    │ 查看硬件详情/盘位图    │
                    │ 勾选 + 设置变量        │
                    │ "下发装机任务"         │
                    └───────────┬───────────┘
                                ▼
                     ┌────────────────────────┐
                     │ 1. 创建 Host 记录      │
                     │ 2. DiscoveredDevice →  │
                     │    Status=adopted      │
                     │ 3. 评估 MatchPolicy    │
                     │ 4. 创建 JobTask        │
                     │ 5. WOL / PXE 重启指令  │
                     └──────────┬─────────────┘
                                ▼
                     ┌────────────────────────┐
                     │ 设备重启               │
                     │ PXE 启动               │
                     │ Server: 有活跃 JobTask │
                     │ → 进入正常执行流程     │
                     └────────────────────────┘
```

---

## 8. 共存策略

```
PXE 启动 /boot/ipxe/script?mac=xx
  │
  ▼
MAC 有活跃 JobTask?
  ├─ 是 → 返回 JobPlan 的 iPXE 脚本 (跳过 Profile/BootMenu)
  └─ 否 → MAC 已注册?
       ├─ 是 → 查 MatchPolicy
       │   ├─ 匹配 → 创建 JobTask，返回对应 iPXE 脚本
       │   └─ 不匹配 → 传统 Profile → BootMenu 逻辑
       └─ 否 → 返回 BootOS 发现脚本
```

---

## 9. 完整 API 端点总览

### 管理端（需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| CRUD | `/api/v1/boot-templates` | 引导模板 |
| CRUD | `/api/v1/raid-configs` | RAID 配置 |
| CRUD | `/api/v1/script-templates` | 脚本模板 |
| CRUD | `/api/v1/job-plans` | 运维计划 |
| CRUD | `/api/v1/match-policies` | 匹配策略 |
| POST | `/api/v1/match-policies/evaluate` | 模拟匹配 |
| CRUD | `/api/v1/job-tasks` | 任务管理 |
| GET | `/api/v1/job-tasks/{id}` | 任务详情（含进度）|
| POST | `/api/v1/job-tasks/{id}/cancel` | 取消任务 |
| POST | `/api/v1/job-tasks/{id}/retry` | 重试 |
| POST | `/api/v1/job-tasks/{id}/skip-stage` | 跳阶段 |
| GET | `/api/v1/discovered-devices` | 发现设备列表 |
| POST | `/api/v1/discovered-devices/{id}/adopt` | 纳入管理 |
| POST | `/api/v1/discovered-devices/{id}/ignore` | 忽略 |
| POST | `/api/v1/discovered-devices/batch-adopt` | 批量纳入 |
| POST | `/api/v1/hosts/import-csv` | CSV 导入 |
| POST | `/api/v1/hosts/batch-install` | 批量创建任务 |
| GET | `/api/v1/raid-configs/{id}/preview` | RAID 预览 |

### Agent 回调（无需认证，Token 鉴权）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/discovery` | 查询发现状态 |
| POST | `/api/v1/discovery/inventory` | 上报硬件信息 |
| GET | `/api/v1/jobs/by-mac/{mac}` | 拉取任务信息 |
| POST | `/api/v1/jobs/{id}/inventory` | 上报安装中硬件 |
| POST | `/api/v1/jobs/{id}/stage/{order}/heartbeat` | 心跳+进度 |
| POST | `/api/v1/jobs/{id}/stage/{order}/done` | 阶段完成 |
| POST | `/api/v1/jobs/{id}/stage/{order}/fail` | 阶段失败 |
| POST | `/api/v1/jobs/{id}/log` | 实时日志 |

### PXE 运行时（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/boot/ipxe/script?mac=&arch=` | 统一 iPXE 入口 |
| GET | `/boot/ipxe/menu?mac=` | 传统菜单 |
| GET | `/boot/*` | 静态文件 |

---

## 10. 前端页面规划

| 页面 | 功能 |
|------|------|
| **发现设备** | 新设备列表、硬件详情（盘位图）、adopt/ignore |
| **运维计划** | JobPlan 列表 + 拖拽 Stage 编排 + 类型筛选 |
| **引导模板** | BootTemplate CRUD |
| **RAID 配置** | RAID 配置编辑器（声明式 + Raw CLI 兜底）|
| **脚本模板** | 脚本编辑器 + 变量声明 + 语法高亮 |
| **匹配策略** | 条件编辑器 + 优先级 + 关联 Plan + 模拟器 |
| **任务列表** | 全部任务 / 按类型筛选 / 状态 / 进度条 + 批量操作 |
| **任务详情** | 阶段进度、日志流、硬件信息、变量列表 |
| **硬件资产** | Host 列表 + 硬件详情（从 Agent 上报）|
| **批量导入** | CSV 上传 + 预览 + 确认 + 执行 |
| **历史记录** | 已完成任务查询 + 统计 |

---

## 11. 完整场景 YAML

### 场景：Dell R740 批量装 10 台 RHEL 9

```yaml
boot_templates:
  - id: "bootos-v1"
    name: "PxeLab BootOS"
    boot_type: kernel
    kernel: "http://{{.Server}}/boot/bootos/vmlinuz"
    initrd: "http://{{.Server}}/boot/bootos/initrd.img"
    cmdline: "PXELAB_SERVER={{.Server}} PXELAB_TASK_ID={{.TaskID}} PXELAB_TOKEN={{.Token}} PXELAB_MODE={{.Mode}} console=ttyS0,115200"
    arch: "amd64"

  - id: "rhel9-kernel"
    name: "RHEL 9 Installer"
    boot_type: kernel
    kernel: "http://mirror.internal/el9/vmlinuz"
    initrd: "http://mirror.internal/el9/initrd.img"
    cmdline: "inst.repo=http://mirror.internal/el9 inst.ks={{.AnswerURL}} console=ttyS0,115200"
    arch: "amd64"

raid_configs:
  - id: "r740-os-data"
    name: "R740 SSD RAID1 + HDD RAID5"
    controller_match:
      vendor: "Dell"
      model: "PowerEdge R740"
    arrays:
      - raid: 1
        drives:
          selector: "first:2,media:ssd,size:>=480"
        name: "OS"
      - raid: 5
        drives:
          selector: "remaining"
        name: "DATA"
    hot_spare:
      drives:
        selector: "first:1,media:hdd"

script_templates:
  - id: "inventory-report"
    name: "硬件信息上报"
    phase: pre-install
    content: |
      #!/bin/sh
      dmidecode -t system 2>/dev/null
      lsblk -o NAME,SIZE,TYPE,MODEL
    builtin: true

  - id: "install-docker"
    name: "安装 Docker"
    phase: post-install
    os: "el"
    content: |
      #!/bin/bash
      dnf install -y docker-ce docker-ce-cli containerd.io
      systemctl enable --now docker

  - id: "configure-network"
    name: "网卡配置"
    phase: post-install
    os: "el"
    content: |
      #!/bin/bash
      nmcli con mod {{.Vars.nic1}} ipv4.addresses {{.Vars.ip}}/{{.Vars.netmask}}
      nmcli con mod {{.Vars.nic1}} ipv4.gateway {{.Vars.gateway}}
      nmcli con mod {{.Vars.nic1}} ipv4.dns {{.Vars.dns}}
      nmcli con up {{.Vars.nic1}}

job_plans:
  - id: "rhel9-dell-prod"
    name: "RHEL 9 生产部署"
    type: os-install
    estimated_duration: 45
    variables:
      timezone: "Asia/Shanghai"
      dns: "10.0.0.1"
    stages:
      - order: 1
        name: "RAID 配置"
        phase: pre-install
        boot_template_id: "bootos-v1"
        raid_config_id: "r740-os-data"
        script_ids: ["inventory-report"]
        timeout: 20

      - order: 2
        name: "安装 RHEL 9"
        phase: install
        boot_template_id: "rhel9-kernel"
        answer_template_id: "rhel9-server-ks"
        timeout: 60

      - order: 3
        name: "首次启动配置"
        phase: post-install
        script_ids: ["configure-network", "install-docker"]
        timeout: 30
        on_failure: continue
        when: "{{.Vars.install_docker}}"

match_policies:
  - id: "pol-r740-rhel"
    name: "Dell R740 RHEL"
    priority: 100
    conditions:
      vendor: "Dell"
      model: "PowerEdge R740"
      labels: ["rhel9"]
    job_plan_id: "rhel9-dell-prod"
    vars:
      has_raid: "true"
      install_docker: "true"
```

CSV 批量导入：

```csv
mac,hostname,labels,install_vars
00:1A:64:01:01,web-01,"rhel9","hostname=web-01,ip=10.0.1.50/24,gateway=10.0.1.1,nic1=eno1"
00:1A:64:01:02,web-02,"rhel9","hostname=web-02,ip=10.0.1.51/24,gateway=10.0.1.1,nic1=eno1"
00:1A:64:01:03,web-03,"rhel9","hostname=web-03,ip=10.0.1.52/24,gateway=10.0.1.1,nic1=eno1"
```

---

## 12. RAID 配置引擎

### 12.1 四层架构

```
Layer 0: UI 可视化选盘 (用户友好)
  Agent 探测 → Server 展示盘位图
  管理员点选/拖拽 → 生成声明式 DSL

Layer 1: 声明式 DSL (灵活)
  drives:
    selector: "first:2,media:ssd"

Layer 2: 探测 + 盘匹配 (自动翻译)
  Agent 读 storcli64 输出 → 盘清单
  按 selector 匹配 → 输出目标盘列表
  匹配失败 → 降级到 Layer 3

Layer 3: Raw CLI (兜底)
  fallback:
    commands: ["storcli64 /c0 add vd type=raid1 drives=0:0,0:1"]
```

### 12.2 执行安全规则

```
1. 预览必选 — 任何写操作前打印/展示完整命令列表
2. 配置备份 — 执行前 dump 当前配置到 Server
3. 幂等检查 — 检测目标盘是否已有 RAID/数据
4. 二次确认 — 生产环境需 UI 确认
5. 超时保护 — 单条命令超时 300s
6. 失败保留 — 保留备份配置供回滚
```

### 12.3 硬件拓扑流程

```
Explorer 扫描:
  storcli64 /c0 /eall /sall show → 盘清单
  → 翻译为统一 DiskInfo 结构:
    {enclosure, slot, type, size, model, status}

Selector 匹配:
  "first:2,media:ssd,size:>=480"
  → 过滤 type=ssd AND size>=480
  → 取前 2 个
  → 结果: [0:0, 0:1]

Translator 翻译:
  ArraySpec{raid:1, drives:[0:0, 0:1]}  →
  storcli64 /c0 add vd type=raid1 drives=0:0,0:1
```

---

## 13. 固件更新流程

```yaml
job_plans:
  - id: "fw-update-dell-r740"
    name: "Dell R740 固件更新"
    type: firmware
    stages:
      - order: 1
        name: "更新 BIOS"
        phase: firmware
        boot_template_id: "bootos-v1"
        script_ids: ["update-dell-bios"]
        firmware_files:
          - url: "http://repo/dell/BIOS_2.15.exe"
            dest: "/fw/bios.exe"
        timeout: 15

      - order: 2
        name: "更新 BMC"
        phase: firmware
        boot_template_id: "bootos-v1"
        script_ids: ["update-dell-bmc"]
        firmware_files:
          - url: "http://repo/dell/BMC_2.85.exe"
            dest: "/fw/bmc.exe"
        timeout: 15
```

执行流程：
1. BootOS 启动，Agent 拉取任务
2. Agent 先下载 firmware_files 到本地
3. 执行对应脚本（脚本引用 `/fw/bios.exe`）
4. 进度心跳同 pre-install 阶段
5. 完成 → POST done → reboot

固件更新完成可能需要多次重启。下一 Stage 依赖硬件版本到预期值后通过 `when` 条件跳过重试。

---

## 14. 后续扩展方向

| 功能 | 说明 |
|------|------|
| HostGroup | 动态/静态主机组，组级批量操作 |
| 模板 Git 同步 | ScriptTemplate / JobPlan 从 Git 同步 |
| Plan 版本化 | JobPlan 副本 + 版本管理 |
| 合规检查 | 安装完成后自动执行合规扫描 |
| 统计/告警 | 成功率统计 + 失败告警 |
| Agent 自动升级 | BootOS Agent 从 Server 拉取最新版本 |
