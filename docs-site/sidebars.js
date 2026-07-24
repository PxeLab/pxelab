/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  guideSidebar: [
    {
      type: 'doc',
      id: 'getting-started',
      label: '快速开始',
    },
    {
      type: 'doc',
      id: 'architecture',
      label: '架构与概述',
    },
    {
      type: 'category',
      label: '使用指南',
      collapsed: false,
      items: [
        {
          type: 'category',
          label: '概览',
          collapsed: false,
          items: [
            'guides/dashboard',
          ],
        },
        {
          type: 'category',
          label: '基础配置',
          collapsed: false,
          items: [
            'guides/services',
            'guides/files',
            'guides/profiles',
            'guides/answer-templates',
            'guides/os-images',
          ],
        },
        {
          type: 'category',
          label: '管理',
          collapsed: false,
          items: [
            'guides/host-management',
            'guides/access-control',
            'guides/install-tasks',
            'guides/bmc',
            'guides/wol',
            'guides/network-diagnostics',
          ],
        },
        {
          type: 'category',
          label: '监控',
          collapsed: false,
          items: [
            'guides/monitoring',
          ],
        },
        'guides/settings',
        'guides/web-ui',
        'guides/deployment',
      ],
    },
    {
      type: 'doc',
      id: 'troubleshooting',
      label: '故障排查与常见问题',
    },
    {
      type: 'doc',
      id: 'contributing',
      label: '贡献指南',
    },
    {
      type: 'doc',
      id: 'release-notes',
      label: '版本历史',
    },
  ],
  referenceSidebar: [
    {
      type: 'category',
      label: '服务参考',
      collapsed: false,
      items: [
        'reference/tftp',
        'reference/dns',
        'reference/nfs',
        'reference/boot-settings',
        'reference/ipxe-build',
      ],
    },
    {
      type: 'category',
      label: '配置参考',
      collapsed: false,
      items: [
        'reference/config-file',
        'reference/api-reference',
        'reference/environment-variables',
        'reference/logging',
      ],
    },
  ],
};

export default sidebars;
