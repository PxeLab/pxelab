# PxELab 三大功能开发计划

## 总体路线

| 阶段 | 功能 | 预估 |
|---|---|---|
| **Phase 1** | ISO 镜像管理：上传 + 元数据解析 + 镜像库 CRUD + 自动提取/挂载 + 注册 netboot | 4d |
| **Phase 2** | 应答模板增强：语法验证 + 实时预览 + 模板库预设 + 变量编辑器 | 2d |
| **Phase 3** | iPXE 脚本管理：可视化菜单 + 语法高亮 + 版本管理 + Bootloader 管理 | 3d |

---

## Phase 1: OS 镜像管理

### 1.1 数据模型 `internal/models/os_image.go`

```go
type OSImage struct {
    ID           uint       `gorm:"primaryKey"`
    Name         string     `json:"name"`          // 用户指定名称
    Filename     string     `json:"filename"`      // 原始文件名
    Size         int64      `json:"size"`
    Distro       string     `json:"distro"`        // ubuntu, debian, centos, esxi, windows
    Version      string     `json:"version"`       // 22.04.3, 11.7.0, Server 2022
    Arch         string     `json:"arch"`          // amd64, arm64
    Status       string     `json:"status"`        // uploading/validating/ready/error
    MountPoint   string     `json:"mount_point"`   // 挂载路径
    ExtractedTo  string     `json:"extracted_to"`  // 提取后的目录
    Checksum     string     `json:"checksum"`      // SHA256
    ErrorMessage string     `json:"error_message"`
    CreatedAt    time.Time  `json:"created_at"`
    UpdatedAt    time.Time  `json:"updated_at"`
}
```

状态机: `uploading → validating → ready` 或 `validating → error`

### 1.2 Store 接口 `internal/store/store.go`

新增 `OSImageStore` 接口：

```go
type OSImageStore interface {
    ListOSImages(ctx context.Context) ([]models.OSImage, error)
    GetOSImage(ctx context.Context, id uint) (*models.OSImage, error)
    CreateOSImage(ctx context.Context, img *models.OSImage) error
    UpdateOSImage(ctx context.Context, img *models.OSImage) error
    DeleteOSImage(ctx context.Context, id uint) error
}
```

在 `Interface` 中嵌入 `OSImageStore`。

### 1.3 SQLite 实现 `internal/store/sqlite.go`

- `Migrate()` 中 `AutoMigrate(&models.OSImage{})`
- 实现 CRUD 方法

### 1.4 API Handler `internal/api/os_images.go`

核心路由：

```
POST   /api/v1/os-images/upload            — 上传 ISO
GET    /api/v1/os-images                   — 列表
GET    /api/v1/os-images/{id}              — 详情
DELETE /api/v1/os-images/{id}              — 删除
POST   /api/v1/os-images/{id}/extract      — 提取/挂载
POST   /api/v1/os-images/{id}/mount        — 挂载 ISO
POST   /api/v1/os-images/{id}/unmount      — 卸载 ISO
```

### 1.5 ISO 处理引擎 `internal/osimage/engine.go`

- `ValidateISO(path string) (*ISOMeta, error)` — 读取 ISO 9660 信息
- `ExtractISO(src, dest string) error` — 提取 kernel/initrd
- `MountISO(src, mountPoint string) error` — Loop mount
- `UnmountISO(mountPoint string) error`
- `DetectDistro(mountPath string) (distro, version, arch string)`

### 1.6 前端页面 `web/src/pages/OSImages.tsx`

- ISO 上传区（拖拽 + 点击上传，显示进度）
- 镜像库列表（名称/版本/架构/状态/大小/时间）
- 搜索过滤
- 详情面板（元数据、关联的 netboot 条目）
- 删除确认

### 1.7 路由注册

- `handler.go` 中新增 `OSImage *OSImageHandler`
- `RegisterRoutes` 中注册路由
- 上传路径配置到 `cfg.Global.DataDir/isos/`

---

## Phase 2: 应答模板增强

### 2.1 语法验证

- 后端 `POST /api/v1/answer-templates/:id/validate` 调用 `template.Parse`
- 前端保存前自动验证，错误行定位

### 2.2 实时预览

- 后端 `POST /api/v1/answer-templates/:id/preview` 接收 host_id 或 mac + variables，返回渲染结果
- 前端预览面板，选主机/手动输入参数

### 2.3 模板库预设

- 后端内置预设：Ubuntu autoinstall、Debian preseed、CentOS kickstart、ESXi kickstart、Windows autounattend
- 前端「从模板库创建」按钮

### 2.4 变量编辑器增强

- 前端变量表格（key/type/required/default/description）
- 密码字段用 ***** 显示

### 2.5 版本 Diff

- 后端 `GET /api/v1/answer-templates/:id/diff?from=v1&to=v2`
- 前端左右对比视图

---

## Phase 3: iPXE 脚本管理

### 3.1 可视化菜单编辑器

- 拖拽排序
- 树形多级菜单
- 菜单显示条件（arch/group 过滤）

### 3.2 脚本编辑器

- 语法高亮（CodeMirror + iPXE 语法定义）
- 代码片段插入
- 语法校验

### 3.3 脚本版本管理

- Profile 版本自动保存
- 版本 diff / 回滚

### 3.4 Bootloader 版本管理

- 记录 NBP 文件元数据
- 从上游一键下载
- 完整性检查

### 3.5 健康检查

- API 端点检测 bootloader 文件存在性
- TFTP/HTTP 可达性
