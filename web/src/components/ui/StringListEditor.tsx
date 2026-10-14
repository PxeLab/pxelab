import { type FC, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, X } from 'lucide-react'
import { Input } from './FormControls'
import { Button } from './Button'

interface Props {
  value: string[]
  onChange: (v: string[]) => void
  placeholder?: string
  /** 是否允许重复项，默认去重 */
  allowDuplicates?: boolean
}

// 字符串列表编辑器：已选项以 chip 形式展示可逐个移除，下方输入框 + 添加按钮追加。
// 用于 SMB 的 allow_ips / allow_users / write_users 等 string[] 字段。
export const StringListEditor: FC<Props> = ({ value, onChange, placeholder, allowDuplicates = false }) => {
  const { t } = useTranslation()
  const [draft, setDraft] = useState('')

  const add = () => {
    const v = draft.trim()
    if (!v || (!allowDuplicates && value.includes(v))) {
      setDraft('')
      return
    }
    onChange([...value, v])
    setDraft('')
  }

  return (
    <div className="space-y-2">
      {value.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {value.map((item, i) => (
            <span
              key={`${item}-${i}`}
              className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-input border border-border text-xs text-foreground"
            >
              <span className="font-mono">{item}</span>
              <button
                type="button"
                onClick={() => onChange(value.filter((_, idx) => idx !== i))}
                className="text-foreground-muted hover:text-accent-red transition-colors"
              >
                <X size={12} />
              </button>
            </span>
          ))}
        </div>
      )}
      <div className="flex gap-2">
        <div className="flex-1">
          <Input
            size="sm"
            value={draft}
            onChange={e => setDraft(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter') {
                e.preventDefault()
                add()
              }
            }}
            placeholder={placeholder}
          />
        </div>
        <Button variant="secondary" size="sm" onClick={add} className="shrink-0">
          <Plus size={14} /> {t('common.add')}
        </Button>
      </div>
    </div>
  )
}
