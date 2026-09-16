import type { AnswerTemplate, FileStatus, MenuEntry, NetbootDistro, NetbootVersion, Profile } from '../api/client'

// R1 快速装机向导 / R7 批量装机共用的系统就绪判定与工具函数。

// Mirrors backend menuEntryFromVersion: Remote URLs preferred over Local paths.
export function entryMatchesVersion(entry: MenuEntry, v: NetbootVersion): boolean {
  const kernel = v.remote?.kernel ?? v.local?.kernel ?? ''
  const initrd = v.remote?.initrd ?? v.local?.initrd ?? ''
  if (entry.type === 'direct') {
    return !!entry.kernel && entry.kernel === kernel && (entry.initrd ?? '') === initrd
  }
  if (entry.type === 'wds') {
    return !!entry.wim && entry.wim === initrd && (entry.url ?? '') === kernel
  }
  return false
}

// OS family → answer template type. Returns undefined when the family is unknown.
export function inferAnswerType(distroName: string, bootType: string): string | undefined {
  const d = distroName.toLowerCase()
  if (bootType === 'wimboot' || d.includes('windows')) return 'autounattend'
  if (d.includes('ubuntu')) return 'subiquity'
  if (d.includes('debian') || d.includes('devuan') || d.includes('kali')) return 'preseed'
  if (/(rhel|rocky|centos|alma|fedora|openeuler|kylin)/.test(d)) return 'kickstart'
  if (d.includes('suse')) return 'autoyast'
  return undefined
}

export const MAC_RE = /^([0-9a-f]{2}:){5}[0-9a-f]{2}$/

export function normalizeMac(mac: string): string {
  return mac.trim().toLowerCase().replace(/-/g, ':')
}

export function defaultHostname(mac: string): string {
  return `node-${mac.replace(/:/g, '').slice(-6)}`
}

export interface MatchedSystem {
  key: string
  profile: Profile
  distroName: string
  versionCodename: string
  versionName: string
  arch: string
  answerType?: string
  defaultTemplate?: AnswerTemplate
}

// Reverse-match every profile menu entry back to its catalog version, then
// require the matching check-files entry to report has_local (= ready).
export function buildReadySystems(
  profiles: Profile[],
  distros: NetbootDistro[],
  fileStatus: FileStatus[],
  templates: AnswerTemplate[],
): MatchedSystem[] {
  const statusMap = new Map(fileStatus.map(f => [`${f.distro}|${f.version}|${f.arch}`, f.has_local]))
  const out: MatchedSystem[] = []
  for (const p of profiles) {
    const seen = new Set<string>()
    for (const entry of p.menu?.entries ?? []) {
      if (entry.type !== 'direct' && entry.type !== 'wds') continue
      for (const distro of distros) {
        for (const v of distro.versions) {
          if (!entryMatchesVersion(entry, v)) continue
          const dedup = `${p.id}|${distro.name}|${v.codename}|${v.arch}`
          if (seen.has(dedup)) continue
          seen.add(dedup)
          if (statusMap.get(`${distro.name}|${v.name}|${v.arch}`) !== true) continue
          const bootType = v.boot_type || (entry.type === 'wds' ? 'wimboot' : '')
          const answerType = inferAnswerType(distro.name, bootType)
          out.push({
            key: dedup,
            profile: p,
            distroName: distro.name,
            versionCodename: v.codename,
            versionName: v.name,
            arch: v.arch,
            answerType,
            defaultTemplate: answerType ? templates.find(tp => tp.type === answerType) : undefined,
          })
        }
      }
    }
  }
  return out
}
