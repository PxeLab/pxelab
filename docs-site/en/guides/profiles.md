# Profiles

> Manage all boot configuration files (Profiles) that determine each host's boot behavior.

**Docs**: [Boot Config](boot-config.md) | [Architecture Mapping](../reference/boot-settings.md)

---

Page path: `/profiles`

Manage all boot configuration files (Profiles):

- **Profile List**: DataTable display with columns for name, architecture, default status, boot type
- **Create Profile**: Dialog form — select name, architecture (x86_64/arm64/etc.), boot type (direct/chain/sanboot/wds/local/custom) and parameters
- **Edit Profile**: Modify boot parameters
- **Script Versioning**: Auto-saves version snapshots on every custom script modification
  - Version list: View history
  - Diff comparison: Compare current vs historical versions
  - Rollback: Restore to any historical version
- **Create from Netboot**: One-click Profile generation from OS install catalog
