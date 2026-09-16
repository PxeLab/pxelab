# PxeLab「快速装机」体验优化 — 需求文档 v2（定稿）

> 来源：2026-09 竞品对比分析（pxesrv / FOG / Cobbler / CloudBoot）结论中选定的高价值项。
> 本文是需求与验收标准，实现方案以各节"技术锚点"为起点另行出设计。

## 决策记录（2026-09 讨论结论）

- **R2、R3**：按初稿方案执行，无修改。
- **R5（Docker/arm64 发行形态）**：**移除**。单二进制直接执行与容器差异不大，不做。
- **R1 向导**：定为**纯编排层**——只串联本地已有资源（引导记录/系统条目/应答模板/装机任务），缺资源给跳链，不在向导内嵌 store 导入或资源创建。
- **R4 信创**：openEuler（公开镜像）做成导入即用；麒麟/UOS（镜像非公开分发）做成**两段式**——导入模板 + 引导用户上传本地 ISO。
- **R6 Windows**：做到 **B 档**；其中 A 档（ISO→PE 引导条目打通、autounattend/winpeshl 应答注入、Catalog 本地源）已由 `a2c6533` 落地，R6 范围收敛为**驱动包注入 + 首登脚本编排 + 官网文档**。不做 C 档（加域/分区/wim 索引全参数向导化，交由应答模板文件承载）。
- **R7 批量**：批次用 **`batch_id` 标签**模型（不建独立 batch 表）；**失败的机器失败锁定**——再次 PXE 启动不自动重装，人工确认后解锁重试；看板轮询刷新。

## 优先级总览

| # | 需求 | 优先级 | 核心价值 |
|---|------|--------|----------|
| R1 | 快速装机向导 | P0 | 把"装一台机"从理解 4 个概念降为 1 个向导 |
| R2 | 基线执行回执 | P0 | 基线脚本从"盲跑"变成可信赖、可排障 |
| R3 | Webhook 通知 | P0 | 装机完成/失败触达运维（钉钉/飞书/Server酱等） |
| R4 | 信创 OS 内容包 | P1 | 把多架构独家优势显性化（openEuler/麒麟/UOS） |
| R6 | Windows 驱动包 + 首登编排（B 档剩余） | P2 | Windows 装机补齐生产可用最后一公里 |
| R7 | 批量装机 | P2 | 从"一次一台"到"一批一个看板" |

---

## R1 快速装机向导（P0）

### 背景与问题

当前装一台机器需要用户理解并串联四个对象：引导配置（profile）、应答模板、主机（host）、装机任务（install task）。新用户上手成本高，"检测到的机器"（PXE 引导记录认领）与装机主流程之间是断开的。

### 目标

Dashboard 提供"快速装机"入口，一个向导完成：**选机器 → 选系统 → 确认参数 → 开装**。

### 范围边界（已定）

向导是**纯编排层**，只串联本地已有资源：

- "选系统"本质是选择已有 profile（store 导入/本地目录条目已会自动生成 profile），向导不新建 profile；
- 本地没有想装的系统、或该 OS 类型缺应答模板时，给出**跳链**（去 store 导入 / 去应答模板页创建），向导不内嵌导入或创建流程。

### 功能需求

- FR-1.1 入口：Dashboard 显著位置（主按钮）。
- FR-1.2 第一步"选机器"：默认展示**未认领的 PXE 引导记录**列表（MAC/IP/最近引导时间），支持勾选多台（为 R7 预留）；也支持手动输入 MAC 新建。
- FR-1.3 第二步"选系统"：展示本地**已就绪**（内核/initrd 已缓存）的 profile 列表（名称/发行版/架构）；每项标注匹配的默认应答模板（按 OS 类型自动匹配，高级折叠中可换）。
- FR-1.4 第三步"确认"：自动生成主机名（可改，如 `node-<mac 后 6 位>`）、展示将使用的 profile / 应答模板汇总；高级选项折叠（extra cmdline、SN、基线开关）。
- FR-1.5 提交后：自动建档主机（若不存在）、创建装机任务、跳转装机任务页并高亮新任务；机器下次 PXE 启动即进入安装。
- FR-1.6 向导各步可回退；全程 ≤ 3 次必选操作（机器、系统、确认）。

### 技术锚点

- 前端：新增 `web/src/pages/QuickInstall.tsx`（Dashboard 入口 + 向导页）；复用 `getPxeBootRecords` / `claimPxeBootRecord`（`web/src/api/pxeboot.ts`）、profiles、应答模板、装机任务 API。
- 后端：`POST /api/v1/install-tasks` 增加"按 mac 自动建档主机"便捷参数，避免前端串 3 个请求。

### 验收标准

- 一台从未纳管的机器 PXE 启动后出现在引导记录；通过向导选定系统并提交后，该机器下次重启自动进入对应 OS 安装，全程未离开向导页。
- 本地无任何系统条目时，向导给出通往 store 的跳链而非死路。

---

## R2 基线执行回执（P0）

### 背景与问题

基线脚本通过 `/api/v1/baselines/pull.sh|ps1` 聚合下发后**盲跑**（`internal/api/baseline_pull_scripts.go`），平台不知道跑了几条、哪条失败、输出是什么。装机排障只能登机器看。

### 目标

基线执行结果（逐条脚本的成功/失败/退出码/输出尾部/耗时）回传平台，主机详情页可视。

### 语义边界（已定）

- **装机任务状态与基线执行结果相互独立**：安装器跑完 ≠ 基线跑完（基线在首次启动后执行）。装机任务状态不变，基线结果独立展示、独立产生 webhook 事件。
- 回执接口**无鉴权**（按 mac/sn 身份键定位主机，遵循 `global.identity_attr`）；文档注明该边界，内网场景接受。

### 功能需求

- FR-2.1 新增回执接口 `POST /api/v1/baselines/report`：Body `{ script_name, seq, exit_code, duration_ms, output_tail（≤4KB） }`。主机不存在时返回 204 静默丢弃（不阻断装机）。
- FR-2.2 `pull.sh` 生成的聚合脚本中，每条脚本执行后自动追加一次 report 调用（curl/wget 二选一，失败静默）；`pull.ps1` 同理（`try/catch` 捕获后上报）。report 接口不可达不影响安装。
- FR-2.3 存储：新增 `baseline_reports` 表（host_id, script_name, seq, exit_code, duration_ms, output_tail, created_at），按主机保留最近 100 条，超出滚动清理。
- FR-2.4 主机详情页新增"基线执行记录"卡片：逐条列出脚本名、结果（绿/红）、耗时，可展开看输出尾部；汇总"3/4 成功"。
- FR-2.5 查询接口 `GET /api/v1/hosts/:id/baseline-reports`。

### 技术锚点

- `internal/api/baseline_pull_scripts.go`：`PullShellScript` / `PullPowerShellScript` 生成逻辑；report URL 与 `buildPullURL` 同源。
- 新增 `internal/models/baseline_report.go` + store 方法；路由挂在现有匿名区（与 pull.sh 同级）。
- 前端 `web/src/pages/HostDetail.tsx` 新增卡片。

### 验收标准

- 主机基线含 2 条 shell 脚本（一条 `exit 0`、一条 `exit 1` 且输出 "boom"），装完后主机详情显示 1 成功 1 失败，失败条可见 "boom" 输出尾部。
- report 接口 404/不可达时安装流程不中断、不报错。

---

## R3 Webhook 通知（P0）

### 背景与问题

事件总线（`internal/eventbus`）只在产品内部流转，装机完成/失败无法触达人。运维只能守着装机任务页刷新。

### 目标

全局 webhook：订阅事件类型，向多个 URL POST；兼容钉钉/飞书自定义机器人与通用 webhook。

### 功能需求

- FR-3.1 配置模型（存 config.yaml `notify.webhooks`，与现有设置体系一致；管理只走 UI）：`[{ id, name, url, events: [...], format: "generic"|"dingtalk"|"feishu", secret?, enabled }]`。
- FR-3.2 事件源（首版）：装机任务完成、装机任务失败、基线脚本执行失败（依赖 R2）、新机器首次 PXE 引导。事件体统一 `{ event, time, host: {mac,name,ip}, detail }`。
- FR-3.3 format：generic 发原始 JSON；dingtalk/feishu 封装为各自机器人文本消息格式（飞书可选签名校验）。
- FR-3.4 投递：异步 worker，超时 5s，失败重试 3 次（指数退避），最终失败记审计日志；不阻塞事件产生方。
- FR-3.5 设置"服务配置"组新增"通知"页：webhook 增删改、启用开关、"发送测试消息"按钮（`POST /api/v1/settings/webhooks/:id/test`）。

### 技术锚点

- 订阅 `internal/eventbus`；投递器放 `internal/notify/`（新包）。
- 接口：`/api/v1/settings/webhooks` CRUD + test。

### 验收标准

- 配置飞书机器人 webhook 并订阅"装机完成"，一台机器装完后 10 秒内群收到含主机名/MAC/系统的消息。
- webhook URL 不可达时装机流程无任何影响，审计日志可见投递失败记录。

---

## R4 信创 OS 内容包（P1）

### 背景与问题

pxelab 已具备 ARM64/RISC-V/LoongArch 引导和 Secure Boot 链路，这是 Cobbler/FOG/CloudBoot 的共同盲区；但 store（hub.pxelab.com）上若没有信创 OS 内容，优势是隐形的。

### 目标

store 上线信创 OS 的引导模板 + 应答模板内容包。

### 分发形态（已定）

- **openEuler**（镜像公开）：netboot_distro 条目直接可用，导入即装（x86_64 / aarch64）。
- **麒麟 Kylin V10 / 统信 UOS**（镜像非公开分发）：**两段式**——store 提供引导模板与应答模板（标注适用的镜像版本），用户导入后按向导上传本地 ISO/指定内核与 initrd 路径后使用。内容条目中明确标注"需自备安装镜像"。

### 功能需求

- FR-4.1 首批内容：openEuler（x86_64/aarch64）直接可用条目；Kylin V10（x86_64/aarch64）、UOS（x86_64/aarch64）两段式条目。麒麟/UOS 均为 anaconda 系，应答走 kickstart。
- FR-4.2 每条内容标注架构、镜像来源（公网 URL / 需自备）与验证状态（已实测/社区贡献）。
- FR-4.3 store 前端对"需自备镜像"的条目在导入后给出明确的补齐指引（跳转 OS 镜像/文件管理页）。
- FR-4.4 官网 docs 增"信创装机"指南页（含 LoongArch 引导注意事项）。

### 技术锚点

- store 仓库（D:\NewCB\store）现有 PS1 生成脚本与 schema，按 netboot_distro 格式新增条目。
- 注意麒麟的 images/pxeboot 路径与 CentOS 系略有差异；两段式条目需要 schema 支持"镜像待补齐"标记（可能需小改 store schema 与本地导入逻辑）。

### 验收标准

- 从 store 导入 openEuler aarch64 条目后，aarch64 机器（或 QEMU 模拟）可完成 PXE 装机。
- 导入麒麟条目后，界面明确提示需自备镜像并引导补齐；补齐 x86_64 镜像后可装机。

---

## R6 Windows 装机链路深化（P2，B 档剩余部分）

### 现状（a2c6533 已落地，R6 范围据此收敛）

Windows 装机的 A 档（打通）已由 `a2c6533`（feat(netboot): local Windows PE catalog source & answer-injection fixes）完成，形成两条互补路线（详见 `docs/windows-deployment.md`）：

- **路线 A（本地 ISO 离线进 PE）**：OS Images 上传/提取 Windows ISO → 一键"创建 wds Profile"（wimboot + 本机文件，离线可用）→ 绑定主机。
- **路线 B（安装任务无人值守）**：安装任务向引导行注入 `autounattend` / `winpeshl`（PE 内执行 `X:\install.bat`）应答；"设为本地源"把 Catalog 的 Windows PE 条目从 boot.netboot.xyz 改写为本机源（离线 + 应答注入同时成立）。

因此 R6 不再包含"ISO → 可装机条目"向导化与应答注入，**剩余范围仅为下面两项 + 文档**。

### 剩余功能需求

- FR-6.1 **驱动包**：文件管理页约定 `drivers/<名称>/` 目录即一个驱动包；应答模板（或主机）可绑定驱动包；装机后首次登录从平台 HTTP 拉取并 `pnputil /add-driver *.inf /subdirs /install` 安装。
- FR-6.2 **首登脚本编排**：复用 FirstLogonCommands 注入机制（`augmentAutoUnattend`），驱动安装与基线 pull（已有）按序编排进首登流程；与 winpeshl（PE 内）路线互不干扰。
- FR-6.3 文档：将 `docs/windows-deployment.md` 的实测步骤整理进官网用户文档（Windows Server 2019/2022、Win10/11 各一篇）。

### 技术锚点

- `internal/osimage`（ISO 提取）、`internal/netboot/catalog.go` 的 `SetWindowsLocal`（本地源）、`internal/api/baseline_pull_scripts.go` 的 `augmentAutoUnattend`（注入点）。
- 驱动拉取走平台 HTTP 文件服务，首登命令可用已建成的基线注入钩子模板机制（`/services/baseline-hooks`）自定义。

### 验收标准

- 路线 B 装一台 Windows Server 2022：绑定驱动包后，首登自动安装包内 `.inf`，设备管理器无未知设备。
- 官网文档可按步骤复现两条路线。

---

## R7 批量装机（P2）

### 背景与问题

装机任务一次只能建一个；装 20 台机器要点 20 次。FOG 有 group、CloudBoot 有批量看板。

### 目标

多选机器 → 批量创建装机任务 → 批次进度看板。

### 模型与语义（已定）

- **批次 = `batch_id` 标签**：装机任务增加 `batch_id` 字段，批量创建的任务共享同一 id；不建独立 batch 表。批次级操作（取消、失败重试）即对该标签下任务的原子遍历操作。
- **失败锁定**：装机任务失败的机器再次 PXE 启动时**不**重新下发安装引导（防安装循环刷掉装一半的机器），须在详情页人工确认后重试解锁。

### 功能需求

- FR-7.1 主机列表与 PXE 引导记录支持多选；"批量装机"动作弹窗：选一个系统（profile + 应答模板），对选中机器批量建档（未建档的引导记录自动 claim）并创建装机任务（同一 `batch_id`）。
- FR-7.2 `POST /api/v1/install-tasks/batch`：入参机器列表 + 系统参数；逐台校验（已有活跃任务的机器跳过并在响应中列明）。
- FR-7.3 装机任务页增加批次看板视图：按 `batch_id` 分组，显示 总数/待开始/安装中/成功/失败，每台机器一行状态，5s 轮询刷新。
- FR-7.4 批次级操作：全部取消、失败重试（逐台走失败解锁语义）。
- FR-7.5 失败锁定：任务失败后该机器的 PXE 引导返回本地启动/菜单而非安装项；任务详情页"重试"按钮解锁。
- FR-7.6 单台并发约束维持现状（一台机器一个活跃任务），不做排队。

### 技术锚点

- `internal/models.InstallTask` 增 `BatchID`；PXE 引导脚本生成处（按 mac 查任务处）增加失败锁定判断。
- 前端：Hosts.tsx 多选、InstallTasks.tsx 看板视图、R1 向导的机器多选复用。

### 验收标准

- 勾选 3 条引导记录批量装机，看板出现一个批次，3 台机器状态随装机推进实时变化。
- 其中一台强制断电显示失败后，该机器重启不再自动进入安装；从看板点重试后恢复正常装机。

---

## 实施顺序建议

```
R2（回执）→ R3（webhook）→ R1（向导）   ← P0，R3 的"基线失败"事件依赖 R2
R4（信创内容，重在外部内容制作）         ← P1
R1 验收后 → R7（批量，复用向导编排层与多选） ← P2
R6（驱动包+首登编排，已大为瘦身）        ← P2，可随时插入
```
