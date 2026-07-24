import React from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import styles from './index.module.css';

const features = [
  {
    title: '开箱即用',
    description: '单二进制部署，内置 DHCP、TFTP、HTTP、DNS、NFS 服务，无需额外依赖。',
    link: '/docs/getting-started',
  },
  {
    title: '11 种架构',
    description: '支持 x86 BIOS/UEFI、ARM32/64、RISC-V、LoongArch，含 Secure Boot 支持。',
    link: '/docs/reference/boot-settings',
  },
  {
    title: 'Web 管理',
    description: '内置 React SPA 管理界面，中英双语，暗色主题，全功能 REST API。',
    link: '/docs/guides/web-ui',
  },
  {
    title: 'iPXE 引导',
    description: '自定义编译 iPXE，配置驱动决策树，支持 direct/chain/wds/sanboot 引导类型。',
    link: '/docs/guides/boot-config',
  },
  {
    title: 'DHCP 四模式',
    description: 'full/proxy/hybrid/off 四种模式，每个接口独立配置，适应各种网络环境。',
    link: '/docs/guides/dhcp',
  },
  {
    title: '硬件管理',
    description: 'WOL 网络唤醒 + BMC/IPMI 带外管理，支持定时调度和批量操作。',
    link: '/docs/guides/host-management',
  },
];

function Feature({title, description, link}) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center padding-horiz--md padding-vert--lg">
        <h3>{title}</h3>
        <p>{description}</p>
        <Link to={link}>了解更多 →</Link>
      </div>
    </div>
  );
}

export default function HomepageFeatures() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {features.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
