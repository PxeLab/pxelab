import { defineConfig } from 'vitepress'

const rawBase = process.env.VITEPRESS_BASE
const base = rawBase
  ? rawBase.startsWith('/')
    ? rawBase.endsWith('/') ? rawBase : `${rawBase}/`
    : `/${rawBase}/`
  : '/'

const zhSidebar = {
  '/': [
    {
      text: '产品',
      items: [
        { text: '产品定位', link: '/product' },
        { text: '功能特性', link: '/features' },
        { text: '优势能力', link: '/advantages' },
        { text: '架构优势', link: '/architecture-advantages' },
        { text: '常见问题', link: '/faq' },
      ],
    },
  ],
  '/guides/': [
    {
      text: '概览',
      items: [
        { text: '仪表盘', link: '/guides/dashboard' },
      ],
    },
    {
      text: '基础配置',
      items: [
        { text: '服务配置', link: '/guides/services' },
        { text: '文件管理', link: '/guides/files' },
        { text: '引导配置', link: '/guides/profiles' },
        { text: '应答文件模板', link: '/guides/answer-templates' },
        { text: 'DHCP 配置', link: '/guides/dhcp' },
        { text: '引导菜单配置', link: '/guides/boot-config' },
        { text: '网络启动目录', link: '/guides/netboot' },
        { text: 'OS 镜像管理', link: '/guides/os-images' },
      ],
    },
    {
      text: '管理',
      items: [
        { text: '主机管理', link: '/guides/host-management' },
        { text: '访问控制', link: '/guides/access-control' },
        { text: '安装任务', link: '/guides/install-tasks' },
        { text: 'BMC 带外管理', link: '/guides/bmc' },
        { text: 'WOL 网络唤醒', link: '/guides/wol' },
        { text: '网络诊断', link: '/guides/network-diagnostics' },
      ],
    },
    {
      text: '监控',
      items: [
        { text: '监控', link: '/guides/monitoring' },
      ],
    },
    {
      text: '其他',
      items: [
        { text: '设置', link: '/guides/settings' },
        { text: 'Web UI 总览', link: '/guides/web-ui' },
        { text: '部署模式', link: '/guides/deployment' },
      ],
    },
  ],
  '/reference/': [
    {
      text: '服务参考',
      items: [
        { text: 'TFTP 服务', link: '/reference/tftp' },
        { text: 'DNS 服务', link: '/reference/dns' },
        { text: 'NFS 服务', link: '/reference/nfs' },
        { text: '架构映射与 Secure Boot', link: '/reference/boot-settings' },
        { text: 'iPXE 编译', link: '/reference/ipxe-build' },
      ],
    },
    {
      text: '配置参考',
      items: [
        { text: '配置文件', link: '/reference/config-file' },
        { text: 'REST API', link: '/reference/api-reference' },
        { text: '环境变量与 CLI', link: '/reference/environment-variables' },
        { text: '日志配置', link: '/reference/logging' },
      ],
    },
  ],
}

// English sidebar: same structure with /en/ prefix
function enSidebar() {
  const result: Record<string, any[]> = {}
  for (const [key, groups] of Object.entries(zhSidebar)) {
    result[`/en${key}`] = groups.map(g => ({
      text: g.text,
      items: g.items.map((item: any) => ({
        text: item.text,
        link: `/en${item.link}`,
      })),
    }))
  }
  return result
}

export default defineConfig({
  base,
  title: 'PxeLab',
  description: '一体化 PXE 网络引导服务器',
  ignoreDeadLinks: true,

  head: [
    ['meta', { name: 'theme-color', content: '#3b82f6' }],
  ],

  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      themeConfig: {
        nav: [
          { text: '产品', link: '/product', activeMatch: '/product|features|advantages|architecture-advantages|faq' },
          { text: '快速开始', link: '/getting-started', activeMatch: '/getting-started' },
          { text: '使用指南', link: '/guides/dashboard', activeMatch: '/guides/' },
          { text: '参考文档', link: '/reference/api-reference', activeMatch: '/reference/' },
        ],
        sidebar: zhSidebar,
        outline: [2, 3],
        search: { provider: 'local' },
        socialLinks: [
          { icon: 'github', link: 'https://github.com/PxeLab/pxelab' },
        ],
        footer: {
          message: 'PxeLab - 一体化 PXE 网络引导服务器',
          copyright: `Copyright © ${new Date().getFullYear()} PxeLab`,
        },
      },
    },
    en: {
      label: 'English',
      lang: 'en-US',
      title: 'PxeLab',
      description: 'All-in-one PXE network boot server',
      themeConfig: {
        nav: [
          { text: 'Product', link: '/en/product', activeMatch: '/en/product|features|advantages|architecture-advantages|faq' },
          { text: 'Getting Started', link: '/en/getting-started', activeMatch: '/en/getting-started' },
          { text: 'Guides', link: '/en/guides/dashboard', activeMatch: '/en/guides/' },
          { text: 'Reference', link: '/en/reference/api-reference', activeMatch: '/en/reference/' },
        ],
        sidebar: enSidebar(),
        outline: [2, 3],
        search: { provider: 'local' },
        socialLinks: [
          { icon: 'github', link: 'https://github.com/PxeLab/pxelab' },
        ],
        footer: {
          message: 'PxeLab - All-in-one PXE Network Boot Server',
          copyright: `Copyright © ${new Date().getFullYear()} PxeLab`,
        },
      },
    },
  },

  themeConfig: {
    outline: [2, 3],
    search: { provider: 'local' },
  },
})
