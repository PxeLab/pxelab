# PxeGo 设计评审与设计规范

## 评审日期
2026-06-15

## 现有设计原型

| 文件 | 主题 | 风格定位 | 来源 |
|------|------|---------|------|
| `web/design-mockup.html` | 深色主题 | 运维终端风，深海军蓝底 + 霓虹蓝/绿点缀 | frontend-design |
| `web/huashu-mockup.html` | 浅色主题 | 现代 SaaS 风，白/灰底 + 靛蓝主色 | huashu-design |

---

## 1. 方向评估

**推荐：深色主题（design-mockup.html）作为主方向，浅色主题作为备选。**

理由：
- PXE 服务器是运维工具，用户群体是 sysadmins/netops
- 深色主题在服务器监控场景下更耐看、减少长时间使用的视觉疲劳
- 深色背景让状态指示灯（绿/黄/红）更突出，信息层级更清晰
- 浅色主题更适合展示型/报告型页面，可留作 settings/配置页面用

---

## 2. 设计评审：design-mockup.html

### 做的好的 👍

| 项 | 说明 |
|----|------|
| **信息密度适中** | 统计卡片 5 个不拥挤，事件列表 + 主机表左右分栏合理 |
| **颜色语义清晰** | 绿=服务正常、黄=待机、红=错误，一眼可读 |
| **状态指示灯设计** | 服务栏的 `status-dot + label + port` 组合清晰 |
| **IPMI 面板** | 四宫格按钮 + BMC 状态行，操作路径直 |
| **字体选择** | JetBrains Mono 做代码/数据展示，Plus Jakarta Sans 做 UI |
| **布局一致性** | 所有页面统一 card-header / card-body / card-footer 模式 |

### 存在的问题 ⚠️

| 问题 | 严重程度 | 建议 |
|------|---------|------|
| 颜色对比度偏低 | 致命 | 深蓝底 `#0a0c10` + 灰色字 `#9aa0ab` 的对比度约 5.5:1（OK），但 `#5c6270` 的 muted 文字在 `#16181f` 上仅 3.2:1（不满足 AA） |
| 缺少骨架屏加载态 | 重要 | 所有表格和卡片都是直接渲染，没有加载中/空状态 |
| 表格可排序性未体现 | 重要 | 列头没有排序指示器，运维人员需要按列排序 |
| 空状态缺失 | 重要 | 主机列表、事件列表没有空状态展示 |
| 移动端适配弱 | 优化 | 768px 断点只是堆叠，缺 hamburger menu 切侧栏 |
| 事件流缺少暂停功能 | 优化 | 实时日志流应该有 pause/resume 按钮 |

### 对比度改进建议

提高侧栏 muted 文字的对比度：
- 当前：`#5c6270` on `#111318` → 约 3.5:1
- 建议：`#7c8298` on `#111318` → 约 5.2:1 ✅

---

## 3. 设计评审：huashu-mockup.html

### 做的好的 👍

| 项 | 说明 |
|----|------|
| **交互反馈丰富** | IPMI 按钮的点击 toast、hover 动效、active 态都做了 |
| **事件模拟系统** | 4 秒间隔的随机事件生成，展示了实时数据流效果 |
| **过滤交互** | 事件页的 filter-chip 切换动效流畅 |
| **页面过渡动画** | fadeSlideIn 让页面切换不突兀 |
| **代码可维护性** | 纯 JS 数据 + 渲染函数分离，容易迁移到 React |
| **信息层级好** | 浅色底 + 靛蓝 accent，清晰不刺眼 |

### 存在的问题 ⚠️

| 问题 | 严重程度 | 说明 |
|------|---------|------|
| 与运维工具调性不完全匹配 | 中 | 浅色 SaaS 风格偏"营销感"，不是运维工具的"控制感" |
| 深色模式未实现 | 中 | 运维人员常 ssh 在深色终端中工作，浅色 UI 会刺眼 |
| 导航缺少次级指示 | 优化 | 没有面包屑，深层页面（如主机详情）缺少位置感 |

---

## 4. 统一设计系统规范

### 4.1 主色方案（深色主题）

```css
/* 语义化颜色令牌 */
--color-bg-base: #0a0c10;        /* 主背景 */
--color-bg-elevated: #111318;    /* 侧栏/顶栏 */
--color-bg-card: #16181f;        /* 卡片 */
--color-bg-hover: #1c1f2c;       /* 悬停 */
--color-bg-input: #1a1d2e;       /* 输入框 */
--color-border: #232738;         /* 边框 */
--color-border-light: #2e3245;   /* 浅边框 */

--color-text-primary: #e8eaed;
--color-text-secondary: #9aa0ab;
--color-text-muted: #6b7294;     /* 提高对比度后的值 */

/* 语义色 */
--color-accent: #3b82f6;         /* 主色调蓝 */
--color-accent-hover: #2563eb;
--color-success: #22c55e;
--color-warning: #eab308;
--color-danger: #ef4444;
--color-info: #06b6d4;
--color-purple: #a855f7;
--color-orange: #f97316;
```

### 4.2 字体系统

```css
/* 标题/UI */
--font-sans: 'Plus Jakarta Sans', system-ui, sans-serif;
--font-mono: 'JetBrains Mono', 'Fira Code', monospace;

/* Type Scale */
--text-xs: 11px;
--text-sm: 12px;
--text-base: 13px;       /* UI 文字基准（运维工具可偏小）*/
--text-lg: 15px;
--text-xl: 18px;
--text-2xl: 22px;
--text-3xl: 28px;

/* Weight */
--weight-normal: 500;
--weight-semibold: 600;
--weight-bold: 700;
```

### 4.3 间距系统

```css
--space-1: 4px;
--space-2: 8px;
--space-3: 12px;
--space-4: 16px;
--space-5: 20px;
--space-6: 24px;
--space-8: 32px;
--space-10: 40px;

/* 圆角 */
--radius-sm: 6px;
--radius-md: 10px;
--radius-lg: 14px;
```

### 4.4 阴影系统

```css
--shadow-sm: 0 1px 2px rgba(0,0,0,0.3);
--shadow-md: 0 4px 12px rgba(0,0,0,0.4);
--shadow-lg: 0 12px 40px rgba(0,0,0,0.5);
--glow-blue: 0 0 20px -8px rgba(59,130,246,0.3);
--glow-green: 0 0 20px -8px rgba(34,197,94,0.3);
```

---

## 5. React 实现建议

### 组件层级设计

```
src/
├── components/ui/          # 基础原子组件
│   ├── Button/
│   ├── Card/
│   ├── DataTable/
│   ├── Modal/
│   ├── Toast/
│   ├── StatusDot/
│   ├── Tag/
│   ├── Input/
│   ├── Toggle/
│   └── EmptyState/
├── components/layout/      # 布局组件
│   ├── AppShell/
│   ├── Sidebar/
│   ├── TopBar/
│   └── StatusBar/
├── components/domain/      # 业务组件
│   ├── ServiceBadge/
│   ├── StatCard/
│   ├── EventItem/
│   ├── EventFilter/
│   ├── IpmiPanel/
│   ├── FileTree/
│   └── BootHistory/
├── hooks/
│   ├── useSSE.ts           # Server-Sent Events
│   ├── useApi.ts           # API 请求封装
│   └── useTheme.ts         # 主题切换
├── pages/
│   ├── Dashboard.tsx
│   ├── Hosts.tsx
│   ├── HostDetail.tsx
│   ├── Profiles.tsx
│   ├── ProfileEditor.tsx
│   ├── Files.tsx
│   ├── Events.tsx
│   └── Settings.tsx
└── api/
    └── client.ts           # API 客户端
```

### 需要使用的库

| 用途 | 推荐库 |
|------|--------|
| 主题变量 | CSS Variables（无需库） |
| 图标 | Lucide React (heroicons 备选) |
| 表格 | @tanstack/react-table |
| 表单 | React Hook Form |
| 通知 | 自建 Toast 组件 |
| 实时数据 | EventSource（原生） |
| 动画 | tailwind-merge + clsx |

### 关键 UX 建议

1. **所有表格必须有**：排序、搜索、分页、空状态
2. **所有异步操作必须有**：loading skeleton → 数据 → 错误状态
3. **事件流必须有**：暂停/恢复按钮、类型过滤、自动滚动到底部开关
4. **IPMI 操作必须有**：确认弹窗（防止误操作）、操作中 loading 动画、结果反馈
5. **配置修改必须有**：保存/重置/取消、修改提示（未保存标记）
6. **键盘快捷键**：`/` 聚焦搜索、`1-5` 快速切换页面、`r` 刷新

---

## 6. 后续动作

1. [ ] 合并两个原型的优点：用深色主题 + huashu 的交互反馈和事件模拟
2. [ ] 修正对比度问题：提高 muted 文字从 `#5c6270` 到 `#6b7294`
3. [ ] 添加空状态、加载态、错误态组件
4. [ ] 实现数据表格排序功能
5. [ ] 添加移动端 hamburger menu
6. [ ] 事件流添加 pause/resume 控制
