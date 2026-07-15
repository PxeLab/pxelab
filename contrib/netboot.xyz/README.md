# contrib/netboot.xyz

本目录用于存放 netboot.xyz 的 fork 仓库，作为 `pxelab netboot sync` 命令的数据源。

## 首次设置

```bash
git clone https://github.com/netbootxyz/netboot.xyz.git contrib/netboot.xyz
cd contrib/netboot.xyz
git remote add upstream https://github.com/netbootxyz/netboot.xyz.git
```

## 同步上游更新

```bash
cd contrib/netboot.xyz
git pull upstream master
```

然后运行 `pxelab netboot sync` 将更新同步到本地 catalog。
