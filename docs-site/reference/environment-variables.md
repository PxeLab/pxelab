# 环境变量与 CLI 参考

> PxeLab 支持的环境变量、CLI 参数与配置优先级。

**相关文档**: [配置文件参考](config-file.md) | [部署模式](../guides/deployment.md)

---

## CLI 参数

```
pxelab [flags]

Flags:
  --config string       配置文件路径
  --data-dir string     数据目录 (default "~/.pxelab/")
  --log-level string    日志级别: debug/info/warn/error (default "info")
  --mode string         运行模式: app/server ("app" 自动打开浏览器)
```

---

## 配置优先级

命令行参数 > 配置文件 > 环境变量 > 默认值

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1（最高） | `--config` / `--data-dir` 等 CLI 参数 | 命令行显式指定 |
| 2 | `config.yaml` | 配置文件 |
| 3 | 环境变量 | 系统环境变量 |
| 4（最低） | 内置默认值 | 硬编码默认值 |

---

## 常用环境变量

PxeLab 主要通过配置文件管理设置，以下环境变量可用于特殊场景：

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `PXELAB_DATA_DIR` | 数据目录路径 | `~/.pxelab/` |
| `PXELAB_LOG_LEVEL` | 日志级别 | `info` |
| `PXELAB_LISTEN_ADDR` | HTTP 监听地址 | `:8080` |

---

## 数据目录结构

```
~/.pxelab/
├── config.yaml          # 配置文件
├── pxelab.db            # SQLite 数据库
├── boot/                # 引导文件（iPXE、PXELinux、GRUB2 二进制）
│   ├── ipxe.efi
│   ├── undionly.kpxe
│   └── ...
├── netboot/             # 网络启动目录（发行版索引）
├── os-images/           # OS 镜像存储
├── logs/                # 日志文件
│   └── pxelab.log
└── volumes/             # NFS 挂载点数据
```
