# 贡献指南

> 如何参与 PxeLab 项目的开发与贡献。

---

## 开发环境

### 前置要求

- Go 1.23+
- Node.js 20+
- Make（可选）

### 克隆仓库

```bash
git clone https://github.com/user/pxelab.git
cd pxelab
```

### 编译后端

```bash
make build          # 生成 bin/pxelab
```

### 编译前端

```bash
cd web
npm install
npm run dev         # Vite 开发服务器，/api 代理到 :8080
```

### 运行测试

```bash
make test           # go test ./...
```

---

## 代码规范

### 后端

- Go 标准格式（gofumpt）
- 遵循 Go 代码审查意见
- 新增功能需包含测试
- 250 行文件上限，超出需拆分

### 前端

- TypeScript 严格模式
- React 19 + Tailwind CSS 4
- 使用 `components/ui/` 中的公共组件
- 新增 UI 文案必须走 i18n（`locales/` 中英双语）

---

## 提交规范

使用 Conventional Commits：

```
feat(dhcp): 添加 DHCP 预留冲突检测
fix(tftp): 修复架构映射错误
docs: 更新用户手册
refactor(api): 重构 REST API 处理器
```

---

## Pull Request 流程

1. Fork 仓库
2. 创建功能分支：`git checkout -b feat/my-feature`
3. 提交变更
4. 推送分支：`git push origin feat/my-feature`
5. 创建 Pull Request
6. 等待 CI 通过和代码审查

---

## 项目结构

```
pxelab/
├── cmd/pxelab/           # CLI 入口
├── internal/
│   ├── api/              # REST API 处理器
│   ├── boot/             # 引导文件服务
│   ├── config/           # 配置管理
│   ├── dhcp/             # DHCP 服务器
│   ├── dns/              # DNS 服务器
│   ├── httpd/            # HTTP 服务器
│   ├── nfs/              # NFS 服务器
│   ├── tftp/             # TFTP 服务器
│   ├── store/            # 数据存储层
│   └── ...
├── web/                  # 前端（React SPA）
├── docs/                 # 文档
├── design-demos/         # 设计稿
└── Makefile              # 构建命令
```
