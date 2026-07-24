import {themes as prismThemes} from 'prism-react-renderer';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'PxeLab',
  tagline: '一体化 PXE 网络引导服务器',
  favicon: 'img/favicon.ico',

  // 部署到 GitHub Pages
  url: 'https://pxelab.dev',
  baseUrl: '/',

  organizationName: 'PxeLab',
  projectName: 'pxelab',

  onBrokenLinks: 'warn',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  // 国际化
  i18n: {
    defaultLocale: 'zh-CN',
    locales: ['zh-CN', 'en'],
    localeConfigs: {
      'zh-CN': {
        htmlLang: 'zh-CN',
        label: '简体中文',
      },
      en: {
        htmlLang: 'en',
        label: 'English',
      },
    },
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          path: '../docs',
          sidebarPath: './sidebars.js',
          editUrl: 'https://github.com/PxeLab/pxelab/edit/main/docs-site/',
          showLastUpdateTime: true,
          exclude: [
            'design/**',
            'superpowers/**',
            'dev-plan.md',
            'IMPLEMENTATION_PLAN.md',
            'boot-architecture.md',
            'dhcp-modes.md',
            'ipxe-build.md',
            'ipxe-settings-guide.md',
            'VERIFICATION-GUIDE.md',
            'sanboot.md',
            '*.html',
            '*_files/**',
          ],
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: {
        defaultMode: 'dark',
        disableSwitch: false,
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'PxeLab',
        logo: {
          alt: 'PxeLab Logo',
          src: 'img/logo.svg',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'guideSidebar',
            position: 'left',
            label: '文档',
          },
          {
            type: 'docSidebar',
            sidebarId: 'referenceSidebar',
            position: 'left',
            label: '参考',
          },
          {
            href: 'https://github.com/PxeLab/pxelab',
            label: 'GitHub',
            position: 'right',
          },
          {
            type: 'localeDropdown',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: '文档',
            items: [
              { label: '快速开始', to: '/docs/getting-started' },
              { label: '架构与概述', to: '/docs/architecture' },
              { label: 'DHCP 配置', to: '/docs/guides/dhcp' },
              { label: 'Web UI 指南', to: '/docs/guides/web-ui' },
            ],
          },
          {
            title: '参考',
            items: [
              { label: 'REST API', to: '/docs/reference/api-reference' },
              { label: '配置文件', to: '/docs/reference/config-file' },
              { label: '故障排查', to: '/docs/troubleshooting' },
            ],
          },
          {
            title: '社区',
            items: [
              { label: 'GitHub', href: 'https://github.com/PxeLab/pxelab' },
              { label: '贡献指南', to: '/docs/contributing' },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} PxeLab. Built with Docusaurus.`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
        additionalLanguages: ['bash', 'yaml', 'json', 'ini', 'go'],
      },
      docs: {
        sidebar: {
          hideable: true,
        },
      },
    }),
};

export default config;
