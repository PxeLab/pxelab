# Troubleshooting & FAQ

> Common issues, log analysis, and FAQ.

**Docs**: [Web UI Guide](guides/web-ui.md) | [Config Reference](reference/config-file.md)

---

## Common Issues

| Symptom | Possible Cause | Troubleshooting Steps |
|---------|---------------|----------------------|
| Client can't PXE boot | DHCP not configured / network unreachable | 1. Check if PxeLab is running 2. Check client and server are on same VLAN 3. Check firewall allows UDP 67 |
| Client boots directly to local disk | Default menu only has local entry | Check default menu config, ensure network boot entries exist |
| Client can't see install catalog | Netboot not enabled | Check "Enable OS install catalog menu" and "Profile menu behavior" |
| DHCP Offer rejected by UEFI firmware | Missing Option 53 | Ensure PxeLab version includes Option 53 fix |
| iPXE reboot loop | PXE_STACK cache issue | Already fixed by PxeLab's custom iPXE build (PXE_STACK disabled) |
| vmxnet3 virtual NIC boot failure | TXE:1 compatibility issue | Use undionly.kpxe (UNDI interface) |
| Config changes not taking effect | Browser cache | Hard refresh (Ctrl+F5) |
| NFS mount fails (Windows) | Path separator issue | Ensure using latest PxeLab version (path.Clean fixed) |
| DNS not resolving | Upstream DNS not configured | Check upstream field in DNS settings |

---

## Log Analysis

```bash
# View real-time logs
# Web UI: Logs page → Select service filter

# Or check log files
tail -f ~/.pxelab/logs/pxelab.log

# Filter by service
grep "DHCP" ~/.pxelab/logs/pxelab.log
```

---

## Network Diagnostics

Built-in network diagnostics in Web UI:
- **Ping** — Test network connectivity
- **Traceroute** — Trace route path
- Supports streaming output, real-time results

API:
```bash
# Ping
curl -X POST http://localhost:8080/api/v1/network/ping \
  -H "Content-Type: application/json" \
  -d '{"host": "192.168.1.1", "count": 4}'

# Traceroute
curl -X POST http://localhost:8080/api/v1/network/traceroute \
  -H "Content-Type: application/json" \
  -d '{"host": "192.168.1.1"}'
```

---

## FAQ

### Q: What permissions does PxeLab need?

**A**: Linux/macOS requires root privileges (DHCP port 67 is a privileged port). Windows requires administrator privileges.

### Q: Can I run multiple DHCP servers simultaneously?

**A**: Yes. Use `proxy` mode to overlay onto existing DHCP, or `hybrid` mode for intelligent routing.

### Q: What OS installations are supported?

**A**: Via iPXE boot:
- Linux: Ubuntu, Debian, CentOS, Fedora, Arch, Gentoo, and 50+ other distros
- Windows: Via WDS emulation or wimboot
- BSD: FreeBSD, OpenBSD, NetBSD
- Live CD: Kali, GParted, SystemRescue, etc.

### Q: How to customize the boot menu?

**A**: Three ways:
1. **Default menu** — Sidebar bottom **Settings → Boot Menu** → Default boot menu
2. **Profile** — Create dedicated boot config for hosts
3. **Custom script** — Sidebar bottom **Settings → Netboot → Custom iPXE Script** (advanced)

### Q: Where is data stored?

**A**: Default location `~/.pxelab/`:
- `config.yaml` — Configuration file
- `pxelab.db` — SQLite database
- `boot/` — Boot files
- `netboot/` — Netboot catalog
- `logs/` — Log files

### Q: How to backup data?

**A**: Back up the entire `~/.pxelab/` directory. Core data is in `pxelab.db` and `config.yaml`.

### Q: Is Docker deployment supported?

**A**: Not yet, on the roadmap. Currently recommended to run the binary directly.

### Q: How to upgrade PxeLab?

**A**:
1. Stop the currently running PxeLab
2. Download the new version binary and replace the old file
3. Data directory `~/.pxelab/` needs no changes — new version auto-migrates
4. Restart

### Q: How to configure multiple NICs?

**A**: Configure multiple interfaces in the `interfaces` section of `config.yaml`, each with independent DHCP mode and subnet settings.
