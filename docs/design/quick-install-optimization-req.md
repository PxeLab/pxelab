# PxeLab「快速装机」体验优化 — 需求文档 v1

> 来源：2026-09 竞品对比分析（pxesrv / FOG / Cobbler / CloudBoot）结论中选定的高价值项。
> 范围：P0 全部 + P1-4（信创内容）+ P1-6（发行形态）+ P2-7（Windows 链路）+ P2-8（批量装机）。
> 本文是需求与验收标准，实现方案以各节"技术锚点"为起点另行出设计。

## 优先级总览

| # | 需求 | 优先级 | 核心价值 |
|---|------|--------|----------|
| R1 | 快速装机向导 | P0 | 把"装一台机"从理解 4 个概念降为 1 个向导 |
| R2 | 基线执行回执 | P0 | 基线脚本从"盲跑"变成可信赖、可排障 |
| R3 | Webhook 通知 | P0 | 装机完成/失败触达运维（钉钉/飞书/Server酱等） |
| R4 | 信创 OS 内容包 | P1 | 把多架构独家优势显性化（麒麟/UOS/openEuler） |
| R5 | 发行形态扩展 | P1 | Docker 镜像 + linux/arm64，进入 VM/容器/ARM 盒子场景 |
| R6 | Windows 装机链路深化 | P2 | Windows 装机体验对齐 Linux |
| R7 | 批量装机 | P2 | 从"一次一台"到"一批一个看板" |

---

## R1 快速装机向导（P0）

### 背景与问题

当前装一台机器需要用户理解并串联四个对象：引导配置（profile）、应答模板、主机（host）、装机任务（install task）。新用户上手成本高，"检测到的机器"（PXE 引导记录认领）与装机主流程之间是断开的。

### 目标

Dashboard 提供"快速装机"入口，一个向导完成：**选机器 → 选系统 → 确认参数 → 开装**。后台自动完成 profile 匹配/创建、应答模板选择与渲染、主机建档、装机任务创建。

### 功能需求

- FR-1.1 向导第一步"选机器"：默认展示**未认领的 PXE 引导记录**列表（MAC/IP/最近引导时间），支持勾选多台（为 R7 批量预留）；也支持手动输入 MAC 新建。
- FR-1.2 第二步"选系统"：展示本地 netboot 目录中**已就绪**（内核/initrd 已缓存）的发行版 + 版本 + 架构；每项标注匹配的默认应答模板。
- FR-1.3 第三步"确认"：自动生成主机名（可改，如 `node-<mac 后 6 位>`）、展示将要使用的 profile / 应答模板 / 基线开关汇总；高级选项折叠（extra cmdline、SN、profile 指定）。
- FR-1.4 提交后：自动建档主机（若不存在）、创建装机任务、跳转装机任务页并高亮新任务；返回的机器下次 PXE 启动即进入安装。
- FR-1.5 向导各步可回退；任一资源缺失（如该发行版无应答模板）时给出**就地创建**的跳转，而非报错终止。

### 非功能需求

- 不引入新的后端概念，向导只是现有四个 API 的编排层；所有中间产物（主机、任务）在详情页可见可改。
- 向导全程 ≤ 3 次点击必选操作（机器、系统、确认）。

### 技术锚点

- 前端：新增 `web/src/pages/QuickInstall.tsx`（或 Dashboard 内嵌向导 Modal）；复用 `getPxeBootRecords` / `claimPxeBootRecord`（`web/src/api/pxeboot.ts`）、netboot 目录、应答模板、装机任务 API。
- 后端：可能需要 `POST /api/v1/install-tasks` 支持"按 mac 自动建档主机"的便捷参数，避免前端串 3 个请求。

### 验收标准

- 一台从未纳管的机器 PXE 启动后出现在引导记录；通过向导选定系统并提交后，该机器下次重启自动进入对应 OS 安装，全程未离开向导页。
- 向导中不暴露 profile/baseline 等概念名词也能完成装机。

---

## R2 基线执行回执（P0）

### 背景与问题

基线脚本通过 `/api/v1/baselines/pull.sh|ps1` 聚合下发后**盲跑**（`internal/api/baseline_pull_scripts.go`），平台不知道跑了几条、哪条失败、输出是什么。装机排障只能登机器看。

### 目标

基线执行结果（逐条脚本的成功/失败/退出码/输出尾部/耗时）回传平台，主机详情页可视。

### 功能需求

- FR-2.1 新增回执接口 `POST /api/v1/baselines/report`：无鉴权但按身份键（mac/sn，遵循 `global.identity_attr`）定位主机；Body：`{ script_name, seq, exit_code, duration_ms, output_tail（≤4KB） }`。机器不存在时返回 204 静默丢弃（不阻断装机）。
- FR-2.2 `pull.sh` 生成的聚合脚本中，每条脚本执行后自动追加一次 report 调用（curl/wget 二选一，失败静默）；`pull.ps1` 同理（`try/catch` 中捕获后上报）。**兼容**：report 接口不可达不影响安装（现有 `|| true` 语义保持）。
- FR-2.3 存储：新增 `baseline_reports` 表（host_id, script_name, seq, exit_code, duration_ms, output_tail, created_at），按主机保留最近 N=100 条，超出滚动清理。
- FR-2.4 主机详情页新增"基线执行记录"卡片：逐条列出脚本名、结果（绿/红）、耗时，可展开看输出尾部；汇总"3/4 成功"。
- FR-2.5 查询接口 `GET /api/v1/hosts/:id/baseline-reports`。

### 技术锚点

- `internal/api/baseline_pull_scripts.go`：`PullShellScript` / `PullPowerShellScript` 生成逻辑；注入 URL 用 `buildPullURL` 同源地址。
- 新增 `internal/models/baseline_report.go` + store 方法；路由挂在现有匿名区内（与 pull.sh 同级）。
- 前端 `web/src/pages/HostDetail.tsx` 新增卡片。

### 验收标准

- 一台主机装机时基线含 2 条 shell 脚本（一条 `exit 0`、一条 `exit 1` 且输出 "boom"），装完后主机详情显示 1 成功 1 失败，失败条可见 "boom" 输出尾部。
- 断网/report 接口 404 时安装流程不中断、不报错。

---

## R3 Webhook 通知（P0）

### 背景与问题

事件总线（`internal/eventbus`）只在产品内部流转，装机完成/失败无法触达人。运维只能守着装机任务页刷新。

### 目标

全局 webhook：订阅事件类型，向一个或多个 URL POST JSON；兼容钉钉/飞书自定义机器人与通用 webhook。

### 功能需求

- FR-3.1 配置模型：`webhooks: [{ id, name, url, events: ["install.finished","install.failed",...], format: "generic"|"dingtalk"|"feishu", secret?, enabled }]`，存 config.yaml（与现有设置体系一致），设置页新增"通知"页管理（增删改、启用开关、测试发送按钮）。
- FR-3.2 事件源：至少覆盖装机任务完成/失败、基线执行失败（依赖 R2）、主机上线（首次 PXE 引导）；事件体统一 `{ event, time, host: {mac,name,ip}, detail }`。
- FR-3.3 投递：异步 worker，超时 5s，失败重试 3 次（指数退避），最终失败记审计日志；不阻塞事件产生方。
- FR-3.4 钉钉/飞书 format 按各自机器人文本消息格式封装（飞书加签名校验可选）。
- FR-3.5 设置页提供"发送测试消息"。

### 技术锚点

- 订阅 `internal/eventbus`；投递器放 `internal/notify/`（新包）。
- 配置：`config.Config` 新增 `notify.webhooks`；接口 `/api/v1/settings/webhooks` CRUD + `POST /api/v1/settings/webhooks/:id/test`。
- 前端：设置"服务配置"组新增"通知"页。

### 验收标准

- 配置飞书机器人 webhook 并订阅"装机完成"，一台机器装完后 10 秒内群收到含主机名/MAC/系统的消息。
- webhook URL 不可达时装机流程无任何影响，审计日志可见投递失败记录。

---

## R4 信创 OS 内容包（P1）

### 背景与问题

pxelab 已具备 ARM64/RISC-V/LoongArch 引导和 Secure Boot 链路，这是 Cobbler/FOG/CloudBoot 的共同盲区；但 store（hub.pxelab.com）上若没有信创 OS 内容，优势是隐形的。

### 目标

store 上线信创 OS 的引导模板 + 应答模板内容包，本地一键导入即可装机。

### 功能需求

- FR-4.1 内容清单（首批）：openEuler（x86_64/aarch64）、麒麟 Kylin V10（x86_64/aarch64）、统信 UOS（x86_64/aarch64/loong64 视镜像可得性）。每个含：netboot_distro 引导模板 + 匹配应答模板（kylin/uos 均为 anaconda 系，走 kickstart；openEuler 同）。
- FR-4.2 每条内容标注架构与验证状态（已实测/社区贡献）。
- FR-4.3 文档：官网 docs 增"信创装机"指南页（含 LoongArch 引导注意事项）。
- FR-4.4 store 仓库（D:\NewCB\store）补充上述 JSON 与生成脚本，部署到 hub.pxelab.com。

### 技术锚点

- store 仓库现有 PS1 生成脚本与 schema（本会话已补全 boot_type 等字段），按现有 netboot_distro 格式新增条目。
- 注意各信创 OS 的镜像源地址有效性与内核/initrd 路径差异（麒麟的 images/pxeboot 路径与 CentOS 略有不同）。

### 验收标准

- 本地实例从 store 导入 openEuler aarch64 条目后，aarch64 机器（或 QEMU 模拟）可完成 PXE 装机。
- store 页面上信创分类下至少 3 个发行版可导入。

---

## R5 发行形态扩展（P1）

### 背景与问题

release 只有 windows/linux amd64 裸二进制。运维环境大量是 VM/容器/NAS/ARM 小主机，没有 Docker 镜像和 arm64 构建会挡掉一批人。

### 功能需求

- FR-5.1 goreleaser 增加 linux/arm64 构建（CGO_ENABLED=0，当前代码已验证跨平台编译干净）。
- FR-5.2 新增 Dockerfile（scratch/alpine 基础，监听 8080 + UDP 67/69 需 host 网络模式说明），goreleaser `dockers` + `docker_manifests` 推送 `pxelab/pxelab:{version,latest}` 到 Docker Hub 与 ghcr.io。
- FR-5.3 文档：README 与官网快速开始增 Docker 运行段落（`--net=host` 与 DHCP 广播的注意事项、配置卷挂载 `/data`）。
- FR-5.4 （可选）docker-compose.yml 示例。

### 技术锚点

- `.github/workflows/release.yml` + `.goreleaser.yaml`（本次会话已建好流水线，矩阵加 arm64、加 dockers 段即可）。
- 注意 UDP 67/69/4011 在容器/bridge 网络下不可用，文档必须写明 host 网络要求。

### 验收标准

- 打一个 rc tag 后 release 页出现 linux_arm64 产物，Docker Hub/ghcr 出现对应 tag 镜像。
- `docker run --net=host` 起的实例在测试机上完成一次 PXE 引导。

---

## R6 Windows 装机链路深化（P2）

### 背景与问题

Windows 装机当前只有 autounattend 应答 + wimboot 菜单项，用户要自己拼 ISO 提取、WinPE、驱动。体验远落后于 Linux 侧"选目录条目即装"。

### 目标

提供"Windows ISO → 可装机条目"的向导化处理，并支持驱动包注入。

### 功能需求

- FR-6.1 OS 镜像页（OSImages）支持上传 Windows ISO 后**一键生成装机条目**：自动提取 install.wim、生成 wimboot 菜单（复用 MenuEntry.wim 链路）、关联默认 autounattend 模板。
- FR-6.2 autounattend 模板变量扩充：计算机名、管理员密码、产品密钥已有；新增**驱动目录**（装机时从平台 HTTP 拉取驱动包并 `pnputil` 安装）与**首登脚本**（已有 FirstLogonCommands 注入机制可复用）。
- FR-6.3 驱动包管理：文件管理页约定 `drivers/<名称>/` 目录即为一个驱动包，应答模板可按主机绑定。
- FR-6.4 文档：Windows Server 2019/2022、Win10/11 各一篇实测指南。

### 技术锚点

- `internal/osimage`（ISO 提取已有基础）、`internal/api/baseline_pull_scripts.go` 的 `augmentAutoUnattend`（注入点已具备）。
- wimboot 文件已在 bootdist 链路内。

### 验收标准

- 上传 Windows Server 2022 ISO，向导生成条目后，一台 UEFI 机器 PXE 启动完成无人值守安装并进桌面。
- 绑定的驱动包内 `.inf` 在首次登录后出现在设备管理器（无未知设备）。

---

## R7 批量装机（P2）

### 背景与问题

装机任务一次只能建一个；装 20 台机器要点 20 次。FOG 有 group、CloudBoot 有批量看板。

### 目标

多选机器 → 批量创建装机任务 → 批次进度看板。

### 功能需求

- FR-7.1 主机列表与 PXE 引导记录支持多选；"批量装机"动作弹窗：选一个系统（发行版+版本+应答模板），对选中机器批量建档（未建档的引导记录自动 claim）并创建装机任务。
- FR-7.2 批次（batch）概念：批量创建的任务携带同一 `batch_id`；装机任务页增加按批次分组的看板视图：总数/待开始/安装中/成功/失败，每台机器一行实时状态（轮询或 WS）。
- FR-7.3 批次级操作：全部取消、失败重试。
- FR-7.4 失败任务在机器下次 PXE 启动时不自动重入安装（防安装循环），需在详情确认后重试。

### 技术锚点

- `internal/models.InstallTask` 增 `BatchID`；`POST /api/v1/install-tasks/batch`。
- 与 `docs/design/install-orchestration-spec.md` 的 JobPlan/JobTask 编排设计对齐——批量装机可视作该编排引擎的最小落地切片，数据模型预留 Stage 扩展空间，避免重复造概念。
- 前端：Hosts.tsx 多选、InstallTasks.tsx 看板视图。

### 验收标准

- 勾选 3 条引导记录批量装机，看板出现一个批次，3 台机器状态随装机推进实时变化；其中一台强制断电后显示失败，可从看板单独重试。

---

## 实施顺序建议

```
R2（回执）→ R3（webhook）→ R1（向导）   ← P0，互相独立，R2 是 R3 的事件源之一
R5（发行形态，半天量，随时插）          ← P1
R4（信创内容，重在外部内容制作）         ← P1
R1 验收后 → R7（批量，复用向导编排层）    ← P2
R6（Windows，依赖最重，最后）            ← P2
```

依赖关系：R3 的"基线失败"事件依赖 R2；R7 复用 R1 的编排接口；R6 与所有项解耦。
