import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { login } from '../api/client'
import { Button } from '../components/ui/Button'

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
    <div className="min-h-screen flex items-center justify-center bg-[var(--bg-base)] px-4 relative overflow-hidden">
      {/* Background gradient decoration */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <div className="absolute -top-1/2 -left-1/4 w-[600px] h-[600px] rounded-full bg-blue-500/5 blur-3xl" />
        <div className="absolute -bottom-1/2 -right-1/4 w-[600px] h-[600px] rounded-full bg-purple-500/5 blur-3xl" />
      </div>

      <div className="w-full max-w-sm relative">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-blue-500 to-indigo-600 flex items-center justify-center font-extrabold text-xl text-white shadow-xl shadow-blue-500/25 mx-auto mb-4">
            PX
          </div>
          <h1 className="text-3xl font-bold text-[var(--text-primary)] tracking-tight">PxeGo</h1>
          <p className="text-sm text-[var(--text-muted)] mt-2">{t('login.title')}</p>
        </div>

        {/* Login Card */}
        <form onSubmit={handleSubmit} className="bg-[var(--bg-card)]/80 backdrop-blur-xl border border-[var(--bg-border)] rounded-2xl p-6 space-y-4 shadow-xl">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
              {t('login.tokenLabel')}
            </label>
            <input
              type="password"
              value={token}
              onChange={e => setToken(e.target.value)}
              placeholder={t('login.tokenPlaceholder')}
              autoFocus
              className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-xl px-4 py-3 text-sm text-[var(--text-primary)] outline-none transition-all placeholder-[var(--text-muted)] focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
            />
          </div>

          {error && (
            <div className="text-sm text-red-400 bg-red-500/10 border border-red-500/20 rounded-xl px-4 py-3">
              {error}
            </div>
          )}

          <Button
            type="submit"
            variant="primary"
            className="w-full justify-center py-3 text-sm"
            disabled={loading || !token.trim()}
          >
            {loading ? t('login.loggingIn') : t('login.submit')}
          </Button>
        </form>

        <p className="text-xs text-center text-[var(--text-muted)] mt-5">
          {t('login.hint')}
        </p>
      </div>
    </div>
  )
}
