import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { login } from '../api/client'

interface LoginProps {
  onLogin: () => void
}

export default function LoginPage({ onLogin }: LoginProps) {
  const { t } = useTranslation()
  const [token, setToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    if (!token.trim()) return

    setLoading(true)
    try {
      await login(token.trim())
      onLogin()
    } catch (err: any) {
      setError(err.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--bg-base)] px-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold text-[var(--text-primary)]">PxeGo</h1>
          <p className="text-sm text-[var(--text-muted)] mt-2">{t('login.title', '管理控制台')}</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-6 space-y-4">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
              {t('login.tokenLabel', 'API 认证令牌')}
            </label>
            <input
              type="password"
              value={token}
              onChange={e => setToken(e.target.value)}
              placeholder={t('login.tokenPlaceholder', '请输入 API 令牌')}
              autoFocus
              className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2.5 text-sm text-[var(--text-primary)] outline-none transition-all placeholder-[var(--text-muted)] focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10"
            />
          </div>

          {error && (
            <div className="text-sm text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading || !token.trim()}
            className="w-full bg-blue-600 hover:bg-blue-500 disabled:bg-blue-600/40 disabled:cursor-not-allowed text-white font-semibold text-sm rounded-lg px-4 py-2.5 transition-colors"
          >
            {loading ? t('login.loggingIn', '登录中...') : t('login.submit', '登录')}
          </button>
        </form>

        <p className="text-xs text-center text-[var(--text-muted)] mt-4">
          {t('login.hint', '首次使用？在服务器终端查看初始令牌，或从本机访问设置页面。')}
        </p>
      </div>
    </div>
  )
}
