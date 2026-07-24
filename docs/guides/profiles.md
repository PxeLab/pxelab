# 引导配置（Profiles）

> 管理所有引导配置文件（Profile），决定每台主机的引导行为。

**相关文档**: [引导配置参考](boot-config.md) | [架构映射](../reference/boot-settings.md)

---

页面路径：`/profiles`

管理所有引导配置文件（Profile）：

- **Profile 列表**：DataTable 展示，列包括名称、架构、是否默认、引导类型
- **创建 Profile**：弹窗表单，选择名称、架构（x86_64/arm64 等）、引导类型（direct/chain/sanboot/wds/local/custom）和对应参数
- **编辑 Profile**：修改引导参数
- **脚本版本管理**：每次修改 custom 类型脚本时自动保存版本快照
  - 版本列表：查看历史版本
  - 差异对比：比较当前脚本与历史版本的差异
  - 回滚：恢复到历史版本
- **从 Netboot 创建**：从 OS 安装目录一键生成 Profile
