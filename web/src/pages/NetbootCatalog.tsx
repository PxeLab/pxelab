import { useState, useEffect } from 'react'
import { Search, HardDrive, Globe, CheckCircle2, XCircle, ChevronRight, ChevronDown, Wrench } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { getNetbootCatalog, getNetbootFileStatus, type NetbootDistro } from '../api/client'

type Tab = 'all' | 'linux' | 'bsd' | 'live' | 'tools'

export default function NetbootCatalog() {
  const [distros, setDistros] = useState<NetbootDistro[]>([])
  const [fileStatuses, setFileStatuses] = useState<Record<string, boolean>>({})
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [activeTab, setActiveTab] = useState<Tab>('all')
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  useEffect(() => { loadData() }, [])

  async function loadData() {
    setLoading(true)
    try {
      const [catRes, fileRes] = await Promise.all([
        getNetbootCatalog(),
        getNetbootFileStatus(),
      ])
      setDistros(catRes.data?.distros || [])
      const statusMap: Record<string, boolean> = {}
      fileRes.data?.forEach(s => {
        statusMap[`${s.distro}/${s.version}/${s.arch}`] = s.has_local
      })
      setFileStatuses(statusMap)
    } catch { /* ignore */ }
    setLoading(false)
  }

  const tabs: { key: Tab; label: string; filter: (d: NetbootDistro) => boolean }[] = [
    { key: 'all', label: '全部', filter: () => true },
    { key: 'linux', label: 'Linux', filter: d => d.menu_group === 'linux' },
    { key: 'bsd', label: 'BSD', filter: d => d.menu_group === 'bsd' },
    { key: 'live', label: 'Live CD', filter: d => d.menu_group === 'live' },
    { key: 'tools', label: '工具', filter: d => d.menu_group === 'tools' },
  ]

  function groupIcon(group: string) {
    switch (group) {
      case 'linux': return <HardDrive size={16} />
      case 'bsd': return <Globe size={16} />
      default: return <Wrench size={16} />
    }
  }

  function filteredDistros() {
    const tab = tabs.find(t => t.key === activeTab)
    return distros.filter(d => {
      if (tab && !tab.filter(d)) return false
      if (search && !d.name.toLowerCase().includes(search.toLowerCase())) return false
      return d.enabled
    })
  }

  function toggleExpand(name: string) {
    setExpanded(prev => ({ ...prev, [name]: !prev[name] }))
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold">OS 安装目录</h1>
        <div className="flex gap-2">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
            <input
              type="text"
              placeholder="搜索..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="pl-8 pr-4 py-1.5 text-sm border border-[var(--border-color)] rounded bg-[var(--bg-primary)]"
            />
          </div>
        </div>
      </div>

      <div className="flex gap-1 mb-4">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-3 py-1.5 text-sm rounded ${
              activeTab === tab.key
                ? 'bg-blue-500 text-white'
                : 'bg-[var(--bg-secondary)] text-[var(--text-muted)] hover:bg-[var(--bg-hover)]'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
        </div>
      ) : (
        <div className="space-y-2">
          {filteredDistros().map(distro => (
            <Card key={distro.name} className="overflow-hidden">
              <button
                onClick={() => toggleExpand(distro.name)}
                className="w-full flex items-center justify-between px-4 py-3 hover:bg-[var(--bg-hover)]"
              >
                <div className="flex items-center gap-3">
                  {expanded[distro.name] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                  {groupIcon(distro.menu_group)}
                  <span className="font-medium">{distro.name}</span>
                  <span className="text-xs text-[var(--text-muted)]">{distro.versions.length} 个版本</span>
                </div>
                {distro.mirror && (
                  <span className="text-xs text-[var(--text-muted)] truncate max-w-[300px]">{distro.mirror}</span>
                )}
              </button>

              {expanded[distro.name] && (
                <div className="border-t border-[var(--border-color)]">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="bg-[var(--bg-secondary)]">
                        <th className="text-left px-4 py-2 font-medium text-[var(--text-muted)]">版本</th>
                        <th className="text-left px-4 py-2 font-medium text-[var(--text-muted)]">架构</th>
                        <th className="text-left px-4 py-2 font-medium text-[var(--text-muted)]">类型</th>
                        <th className="text-center px-4 py-2 font-medium text-[var(--text-muted)]">本地文件</th>
                      </tr>
                    </thead>
                    <tbody>
                      {distro.versions.filter(v => v.enabled).map(ver => {
                        const statusKey = `${distro.name}/${ver.name}/${ver.arch}`
                        const hasLocal = fileStatuses[statusKey] || !!ver.local
                        return (
                          <tr key={`${distro.name}-${ver.codename}`} className="border-t border-[var(--border-color)]">
                            <td className="px-4 py-2 text-[var(--text-primary)]">{ver.name}</td>
                            <td className="px-4 py-2 text-[var(--text-muted)]">{ver.arch}</td>
                            <td className="px-4 py-2 text-[var(--text-muted)]">{ver.install_type || 'legacy'}</td>
                            <td className="px-4 py-2 text-center">
                              {hasLocal
                                ? <CheckCircle2 size={16} className="text-green-500 inline" />
                                : <XCircle size={16} className="text-red-400 inline" />
                              }
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </Card>
          ))}

          {filteredDistros().length === 0 && (
            <div className="text-center py-16 text-[var(--text-muted)]">
              没有匹配的发行版
            </div>
          )}
        </div>
      )}
    </div>
  )
}
