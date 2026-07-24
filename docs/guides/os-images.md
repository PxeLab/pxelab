# OS 镜像管理

> ISO 镜像的上传、挂载、解压与文件浏览。

**相关文档**: [网络启动目录](netboot.md) | [文件管理](web-ui.md) | [REST API 参考](../reference/api-reference.md)

---

## API 端点

```
GET    /api/v1/os-images                # 列出所有镜像
POST   /api/v1/os-images/upload         # 上传 ISO
POST   /api/v1/os-images/import         # 导入本地文件
GET    /api/v1/os-images/{id}           # 获取镜像详情
PUT    /api/v1/os-images/{id}           # 更新元数据
DELETE /api/v1/os-images/{id}           # 删除镜像
POST   /api/v1/os-images/{id}/extract   # 解压 ISO
POST   /api/v1/os-images/{id}/mount     # 挂载 ISO
POST   /api/v1/os-images/{id}/unmount   # 卸载 ISO
POST   /api/v1/os-images/{id}/reprocess # 重新处理
GET    /api/v1/os-images/{id}/file      # 下载文件
```

---

## 功能

- **上传 ISO** — 拖拽或选择上传
- **导入本地文件** — 指定服务器本地路径导入
- **自动检测** — 自动识别发行版类型
- **挂载/卸载** — ISO 挂载点管理
- **文件浏览** — 浏览镜像挂载点内容
- **下载** — 下载镜像文件
- **重新处理** — 重新检测和解析镜像
