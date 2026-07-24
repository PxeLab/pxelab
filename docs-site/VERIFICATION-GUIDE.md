# PxeLab iPXE 架构支持验证指南

本文档指导如何验证 PxeLab 的完整 iPXE 架构支持（含 Secure Boot）功能。

---

## 一、验证前提

| 项目 | 要求 |
|------|------|
| Go | >= 1.23 |
| Node.js | >= 18 |
| Git | 最新 |
| 操作系统 | Windows / Linux / macOS（DHCP 服务需要 root/admin 权限） |
| 网络 | 测试 PXE 启动需要独立网络或 VLAN，避免影响生产环境 |

---

## 二、代码级验证（无需运行服务）

### 2.1 编译验证

```bash
# 后端编译
cd D:\NewCB\PxeGo
go build ./cmd/pxelab/
# 预期：无错误输出，生成 bin/pxelab.exe

# 前端编译
cd web
npm ci
npm run build
# 预期：✓ built in Xs

# 单元测试
go test ./internal/boot/... ./internal/dhcp/... -v
# 预期：全部 PASS（18+ 测试用例）

# 静态分析
go vet ./internal/boot/... ./internal/dhcp/... ./internal/api/...
# 预期：无输出（0 错误）
```

### 2.2 二进制文件完整性验证

验证 `boot/` 目录包含所有 iPXE v2.0.0 发布包中的引导文件：

| 文件名 | 架构 | 用途 | 预期状态 |
|--------|------|------|----------|
| `ipxe.pxe` | x86 BIOS | iPXE 引导 | ✅ 已存在 |
| `ipxe32.efi` | EFI IA32 | iPXE 引导 | ✅ 已存在 |
| `ipxe.efi` | EFI x86-64 | iPXE 引导 | ✅ 已存在 |
| `snponly.efi` | EFI BC | iPXE 引导（SNP 驱动） | ✅ 已存在 |
| `ipxe-arm32.efi` | EFI ARM32 | iPXE 引导 | ✅ 已存在 |
| `ipxe-arm64.efi` | EFI ARM64 | iPXE 引导 | ✅ 已存在 |
| `ipxe-riscv32.efi` | RISC-V 32 | iPXE 引导 | ✅ 已存在 |
| `ipxe-riscv64.efi` | RISC-V 64 | iPXE 引导 | ✅ 已存在 |
| `ipxe-loong64.efi` | LoongArch64 | iPXE 引导 | ✅ 已存在 |
| `ipxe-x86_64-sb.efi` | EFI x86-64 | Secure Boot iPXE | ✅ 已存在 |
| `ipxe-arm64-sb.efi` | EFI ARM64 | Secure Boot iPXE | ✅ 已存在 |
| `shim-x86_64.efi` | EFI x86-64 | Secure Boot Shim | ✅ 已存在 |
| `shim-arm64.efi` | EFI ARM64 | Secure Boot Shim | ✅ 已存在 |

### 2.3 架构映射代码审查

**核心文件清单**（修改/新增的文件）：

| 文件 | 职责 | 关键审查点 |
|------|------|-----------|
| `internal/boot/arch.go` | LoongArch IANA 常量定义 | 常量值 `0x25`/`0x27` 与 IANA 一致 |
| `internal/boot/archmap.go` | 架构→引导文件映射表 | `defaultArchMap()` 包含 11 个架构 |
| `internal/boot/nbp.go` | NBP 选择逻辑 | `ResolveNBP()` 按架构优先级选择 |
| `internal/config/config.go` | `ArchEntry` 配置结构体 | 含 `SecureBoot`/`IPXESB`/`Shim` 字段 |
| `internal/api/settings.go` | REST API 处理器 | `ArchEntryResponse` 含 Secure Boot 字段 |
| `internal/dhcp/handler.go` | DHCP 处理器 | `resolveNBPForClient()` 支持所有架构 |
| `internal/dhcp/options.go` | DHCP 选项工具 | `ArchString()` 覆盖所有架构 |
| `web/src/pages/BootSettings.tsx` | 前端 Boot Settings 页面 | Secure Boot 列显示正确 |
| `web/src/api/client.ts` | 前端 API 类型 | `ArchEntryData` 含新字段 |
| `web/src/locales/zh-CN.json` | 中文翻译 | `secureBoot`/`supported` 翻译正确 |

**逐项审查清单**：

```bash
# 1. 检查 LoongArch IANA 常量
cat internal/boot/arch.go
# 预期：EFI_LOONGARCH32 = 0x25 (37), EFI_LOONGARCH64 = 0x27 (39)

# 2. 检查 defaultArchMap 包含所有架构
cat internal/boot/archmap.go | grep -c "NBP: \"ipxe\""
# 预期：11（每个架构都有默认 NBP）

# 3. 检查 Secure Boot 配置
cat internal/boot/archmap.go | grep "SecureBoot: true"
# 预期：2 行（EFI_X86_64 和 EFI_ARM64）

# 4. 检查 EFI_BC 使用 snponly.efi
cat internal/boot/archmap.go | grep "EFI_BC"
# 预期：IPXE: "snponly.efi"（不是 ipxe.efi）

# 5. 检查 DHCP 架构检测覆盖
cat internal/dhcp/options.go | grep -A2 "EFI_LOONGARCH32\|EFI_ARM32\|RISCV32"
# 预期：所有新架构都有 ArchString() 映射

# 6. 检查前端 i18n
cat web/src/locales/zh-CN.json | grep "secureBoot\|supported"
# 预期：secureBoot: "安全启动", supported: "支持"
```

---

## 三、API 集成验证

### 3.1 启动服务

```bash
cd D:\NewCB\PxeGo
go build -o bin/pxelab.exe ./cmd/pxelab/
mkdir -p /tmp/pxelab-test   # Linux/macOS
# 或 Windows:
mkdir "$env:TEMP\pxelab-test"

# 启动服务（不绑定 DHCP/TFTP，仅验证 API）
.\bin\pxelab.exe --data-dir "$env:TEMP\pxelab-test" --log-level warn
```

### 3.2 测试 ArchMap 默认值端点

```bash
curl -s http://127.0.0.1:8080/api/v1/services/archmap/defaults | python -m json.tool
```

**预期响应**（应包含 10 个架构条目）：

```json
{
  "success": true,
  "data": {
    "entries": [
      {
        "arch_code": 0,
        "arch_name": "Intel x86PC",
        "nbp": "ipxe",
        "chain_load": false,
        "ipxe": "ipxe.pxe",
        "pxelinux": "pxelinux.bios",
        "secure_boot": false,
        "ipxe_sb": "",
        "shim": ""
      },
      {
        "arch_code": 7,
        "arch_name": "EFI x86-64",
        "nbp": "ipxe",
        "ipxe": "ipxe.efi",
        "pxelinux": "pxelinux.efi",
        "grub": "grubx64.efi",
        "secure_boot": true,
        "ipxe_sb": "ipxe-x86_64-sb.efi",
        "shim": "shim-x86_64.efi"
      },
      {
        "arch_code": 9,
        "arch_name": "EFI BC",
        "nbp": "ipxe",
        "ipxe": "snponly.efi",
        "secure_boot": false
      },
      {
        "arch_code": 10,
        "arch_name": "EFI ARM32",
        "nbp": "ipxe",
        "ipxe": "ipxe-arm32.efi",
        "secure_boot": false
      },
      {
        "arch_code": 11,
        "arch_name": "EFI ARM64",
        "nbp": "ipxe",
        "ipxe": "ipxe-arm64.efi",
        "grub": "grubaa64.efi",
        "secure_boot": true,
        "ipxe_sb": "ipxe-arm64-sb.efi",
        "shim": "shim-arm64.efi"
      },
      {
        "arch_code": 25,
        "arch_name": "EFI RISC-V 32-bit",
        "nbp": "ipxe",
        "ipxe": "ipxe-riscv32.efi",
        "secure_boot": false
      },
      {
        "arch_code": 27,
        "arch_name": "EFI RISC-V 64-bit",
        "nbp": "ipxe",
        "ipxe": "ipxe-riscv64.efi",
        "secure_boot": false
      },
      {
        "arch_code": 37,
        "arch_name": "EFI_LOONGARCH32",
        "nbp": "ipxe",
        "ipxe": "ipxe-loong64.efi",
        "secure_boot": false
      },
      {
        "arch_code": 39,
        "arch_name": "EFI_LOONGARCH64",
        "nbp": "ipxe",
        "ipxe": "ipxe-loong64.efi",
        "secure_boot": false
      }
    ]
  }
}
```

### 3.3 验证要点（逐项打勾）

| # | 检查项 | 预期值 | 实际结果 |
|---|--------|--------|----------|
| 1 | 条目总数 | 10 个架构 | ☐ |
| 2 | 所有架构 `nbp` | `"ipxe"` | ☐ |
| 3 | EFI x86-64 `secure_boot` | `true` | ☐ |
| 4 | EFI ARM64 `secure_boot` | `true` | ☐ |
| 5 | EFI x86-64 `ipxe_sb` | `"ipxe-x86_64-sb.efi"` | ☐ |
| 6 | EFI ARM64 `ipxe_sb` | `"ipxe-arm64-sb.efi"` | ☐ |
| 7 | EFI x86-64 `shim` | `"shim-x86_64.efi"` | ☐ |
| 8 | EFI ARM64 `shim` | `"shim-arm64.efi"` | ☐ |
| 9 | EFI BC `ipxe` | `"snponly.efi"`（非 ipxe.efi） | ☐ |
| 10 | ARM32 `arch_code` | `10` | ☐ |
| 11 | RISC-V 32 `arch_code` | `25` | ☐ |
| 12 | LoongArch32 `arch_code` | `37` | ☐ |
| 13 | LoongArch64 `arch_code` | `39` | ☐ |
| 14 | LoongArch64 `ipxe` | `"ipxe-loong64.efi"` | ☐ |

### 3.4 测试 ArchMap 更新端点

```bash
# 更新 x86-64 架构的 NBP 为 pxelinux
curl -s -X PUT http://127.0.0.1:8080/api/v1/services/archmap \
  -H "Content-Type: application/json" \
  -d '{"entries":[
    {"arch_code":7,"arch_name":"EFI x86-64","nbp":"pxelinux","chain_load":true,
     "ipxe":"ipxe.efi","pxelinux":"pxelinux.efi","grub":"grubx64.efi",
     "secure_boot":true,"ipxe_sb":"ipxe-x86_64-sb.efi","shim":"shim-x86_64.efi"}
  ]}'

# 验证更新生效
curl -s http://127.0.0.1:8080/api/v1/services/archmap | python -m json.tool
# 预期：x86-64 条目 nbp="pxelinux", chain_load=true

# 恢复默认值
curl -s -X PUT http://127.0.0.1:8080/api/v1/services/archmap \
  -H "Content-Type: application/json" \
  -d '{"entries":[
    {"arch_code":7,"arch_name":"EFI x86-64","nbp":"ipxe","chain_load":false,
     "ipxe":"ipxe.efi","pxelinux":"pxelinux.efi","grub":"grubx64.efi",
     "secure_boot":true,"ipxe_sb":"ipxe-x86_64-sb.efi","shim":"shim-x86_64.efi"}
  ]}'
```

---

## 四、Web UI 验证

### 4.1 启动前端开发服务器

```bash
cd web
npm run dev
# 预期：Vite 开发服务器启动在 http://localhost:5173
```

### 4.2 UI 验证清单

打开浏览器访问 `http://localhost:5173/settings/boot`：

| # | 检查项 | 预期效果 | 实际结果 |
|---|--------|----------|----------|
| 1 | 架构表格 | 显示 10 行（所有架构） | ☐ |
| 2 | Secure Boot 列 | x86-64 和 ARM64 显示"支持" | ☐ |
| 3 | Secure Boot 列 | 其他架构显示"—" | ☐ |
| 4 | NBP 下拉菜单 | 选择每个架构都能切换 iPXE/PXELinux/GRUB2 | ☐ |
| 5 | 链式加载复选框 | 选择 PXELinux/GRUB2 时可勾选"链式加载到 iPXE" | ☐ |
| 6 | ARM64 + PXELinux | 自动回退提示（ARM64 不支持 PXELinux） | ☐ |
| 7 | 默认值按钮 | 点击后恢复所有架构到默认配置 | ☐ |
| 8 | 保存按钮 | 保存成功后显示成功提示 | ☐ |
| 9 | 中文翻译 | 所有标签和按钮显示中文 | ☐ |
| 10 | 暗色主题 | 在暗色模式下显示正常 | ☐ |

---

## 五、DHCP 功能验证

### 5.1 DHCP 架构检测测试

```bash
# 查看 DHCP 日志中客户端架构检测
# 启动 PxeLab 后，用 PXE 客户端启动
# 日志中应包含：
# "detected arch: EFI x86-64 (7)"
# "resolved NBP: ipxe.efi"

# 验证不同架构的 NBP 选择
# 用对应架构的机器启动，检查日志：
# - x86 BIOS → ipxe.pxe
# - x86 UEFI → ipxe.efi
# - ARM64 → ipxe-arm64.efi
# - LoongArch → ipxe-loong64.efi
```

### 5.2 Secure Boot 链式加载测试

```
测试环境：x86 UEFI 机器，启用 Secure Boot
测试步骤：
1. 设置 x86-64 架构 NBP=pxelinux，chain_load=true
2. 用 UEFI 客户端 PXE 启动
3. 预期启动链：DHCP → shim-x86_64.efi → ipxe-x86_64-sb.efi → pxelinux.efi
4. 验证 Secure Boot 签名校验通过
```

---

## 六、引导文件二进制验证

### 6.1 验证所有引导文件可加载

```bash
# 列出所有引导文件大小（非零即有效）
ls -la boot/ipxe*.efi boot/shim*.efi boot/snponly*.efi boot/ipxe.pxe
# 预期：所有文件大小 > 0

# Windows:
Get-ChildItem boot\ipxe*.efi,boot\shim*.efi,boot\snponly*.efi,boot\ipxe.pxe | Select-Object Name,Length
```

### 6.2 验证嵌入到二进制中

```bash
# 检查 bootdist 是否包含所有引导文件
go run ./cmd/pxelab/ --help
# 如果 webdist/bootdist 嵌入失败，启动时报 embed 错误

# 检查嵌入的文件
go list -m -json | grep -A5 "Embed"
```

---

## 七、预存问题（非本次变更引起）

| 问题 | 文件 | 影响 | 建议 |
|------|------|------|------|
| `tftp/server_test.go` API 不匹配 | `internal/tftp/server_test.go` | 传递 `0` 给 `config.TFTPConfig` 导致编译失败 | 修复为 `config.TFTPConfig{}` |
| `other/ipxeboot/` 目录未加入 `.gitignore` | 项目根目录 | 临时下载文件可能被误提交 | 添加到 `.gitignore` |
| `pxelinux32.efi` 缺失 | `boot/` | EFI IA32 架构使用 pxelinux 时找不到文件 | 从 iPXE 发布包下载 |

---

## 八、验证结果汇总

| 验证类别 | 状态 | 备注 |
|----------|------|------|
| 后端编译 | ☐ | |
| 前端编译 | ☐ | |
| 单元测试 | ☐ | |
| Go Vet | ☐ | |
| 二进制文件完整 | ☐ | |
| API 默认值端点 | ☐ | |
| API 更新端点 | ☐ | |
| Web UI 显示 | ☐ | |
| DHCP 架构检测 | ☐ | |
| Secure Boot 链式加载 | ☐ | |
| PXE 实际启动 | ☐ | |

**验证人**: ________________ **日期**: ________________

---

## 九、测试报告模板

验证完成后，填写以下报告：

```
=== PxeLab iPXE 架构支持验证报告 ===

验证人：
验证日期：
验证环境：
  - 操作系统：
  - Go 版本：
  - Node.js 版本：
  - 测试机器架构：

=== 编译验证 ===
后端编译：PASS/FAIL
前端编译：PASS/FAIL
单元测试：PASS/FAIL (X/Y)
Go Vet：PASS/FAIL

=== API 验证 ===
archmap 端点返回条目数：10/预期10
所有架构 nbp 字段正确：YES/NO
Secure Boot 字段正确：YES/NO
更新端点功能正常：YES/NO

=== UI 验证 ===
页面加载正常：YES/NO
Secure Boot 列显示正确：YES/NO
NBP 切换功能正常：YES/NO
中文翻译完整：YES/NO

=== 功能验证 ===
DHCP 架构检测：PASS/FAIL
Secure Boot 启动链：PASS/FAIL
PXE 实际启动（架构）：

=== 发现的问题 ===
1.
2.

=== 结论 ===
通过/有条件通过/不通过
```
