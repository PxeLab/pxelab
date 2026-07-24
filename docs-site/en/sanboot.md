# sanboot Use Case Reference

| Scenario | ISO Type | sanboot Suitable? | Reason |
|----------|----------|-------------------|--------|
| DOS boot disk (e.g., fdfullcd.iso) | Single runtime env | ✅ | Boots and runs, no external media access |
| Live Linux (Kali/PE-like, RescueCD) | Self-contained runtime | ✅ | Kernel + initramfs includes squashfs, can remount itself from SAN device |
| WinPE maintenance disk (grldr/isolinux boot) | Single runtime env | ✅ (BIOS) | Enters PE then gets tools via wim/network, doesn't depend on ISO source |
| Memtest86+ and bare-metal tools | Single kernel | ✅ | Doesn't access disk after boot |
| iSCSI LUN direct boot (installed Windows/Linux) | Installed system disk | ✅ | Goal is "running system" not "installing system" |
| CentOS/RHEL install ISO | Installer type | ⚠️→❌ | Anaconda needs explicit inst.repo, blind SAN scanning often fails |
| Ubuntu/Debian install ISO | Installer type | ⚠️→❌ | Similarly needs repo=/url=, and subiquity has poor SAN remount support |
| Windows install ISO (boot.wim→setup.exe) | Installer type | ❌ | Needs wim + BCD extraction, use wimboot instead |

**Decision Rule**: After the ISO boots, does it need to find "itself" again to read install files? Yes → Installer type, sanboot is problematic. No → Runtime type, sanboot is ideal.
