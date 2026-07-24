# 部署模式

> PxeLab 支持 Server 模式和 App 模式，适用于不同场景。

**相关文档**: [快速开始](../getting-started.md) | [配置文件参考](../reference/config-file.md)

---

## Server 模式（默认）

```bash
pxelab --mode server
# 或
pxelab   # 默认即 server 模式
```

- 前台运行，日志输出到 stderr
- 适合服务器部署、后台运行
- Linux 下可通过 systemd 管理

---

## App 模式

```bash
pxelab --mode app
```

- 自动打开浏览器
- Windows 下以系统托盘运行（隐藏控制台窗口）
- 适合桌面环境、个人使用

---

## Windows 系统托盘

在 Windows 上，PxeLab 检测到桌面环境时自动启用系统托盘模式：

- 右键托盘图标：打开浏览器 / 打开数据目录 / 退出
- 托盘图标显示服务运行状态
- 关闭浏览器不会停止服务，通过托盘退出才会

---

## systemd 部署（Linux）

创建 systemd 服务文件 `/etc/systemd/system/pxelab.service`：

```ini
[Unit]
Description=PxeLab PXE Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pxelab --mode server
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable pxelab
sudo systemctl start pxelab
```
