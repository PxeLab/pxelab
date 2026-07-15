sanboot 适用场景对照表
场景	ISO 类型	sanboot 是否适合	原因
DOS 启动盘（如 fdfullcd.iso）	单运行环境	✅	启动即运行，不再访问外部介质
Live Linux（Kali/PE-like、RescueCD）	自包含运行环境	✅	内核 + initramfs 自带 squashfs，能从 SAN 设备回挂自身
WinPE 维护盘（grldr/isolinux 引导型）	单运行环境	✅（BIOS）	进 PE 后通过 wim/网络获取工具，不依赖 ISO 安装源
Memtest86+ 等裸机工具	单内核	✅	引导后不读盘
iSCSI LUN 直启已装好的 Windows/Linux	已安装系统盘	✅	目标是"运行系统"而非"安装系统"
CentOS/RHEL 安装 ISO	安装器型	⚠️→❌	Anaconda 需显式 inst.repo，盲扫 SAN 设备常失败
Ubuntu/Debian 安装 ISO	安装器型	⚠️→❌	同理需 repo=/url=，且 subiquity 对 SAN 回挂支持差
Windows 安装 ISO（boot.wim→setup.exe）	安装器型	❌	需提取 wim + BCD，走 wimboot
判断准则：ISO 启动后是否还要回头找"自己"来读安装文件？ 是 → 安装器型，sanboot 易踩坑；否 → 运行型，sanboot 最合适。