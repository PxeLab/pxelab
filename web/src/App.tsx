import { useState, useEffect, Suspense, lazy } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AppShell } from './components/layout/AppShell'
import { ToastProvider } from './components/ui/Toast'
import { checkSession } from './api/client'
import LoginPage from './pages/Login'
import './i18n'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const Hosts = lazy(() => import('./pages/Hosts'))
const HostDetail = lazy(() => import('./pages/HostDetail'))
const Profiles = lazy(() => import('./pages/Profiles'))
const Files = lazy(() => import('./pages/Files'))
const Events = lazy(() => import('./pages/Events'))
const Logs = lazy(() => import('./pages/Logs'))
const Settings = lazy(() => import('./pages/Settings'))
const Services = lazy(() => import('./pages/Services'))
const NetbootCatalog = lazy(() => import('./pages/NetbootCatalog'))
const AnswerTemplates = lazy(() => import('./pages/AnswerTemplates'))
const DNSRecords = lazy(() => import('./pages/DNSRecords'))
const AccessControl = lazy(() => import('./pages/AccessControl'))

function LoadingFallback() {
  return (
    <div className="flex items-center justify-center py-20">
      <div className="flex flex-col items-center gap-3 text-[var(--text-muted)]">
        <div className="w-8 h-8 border-2 border-blue-500/30 border-t-blue-500 rounded-full animate-spin" />
        <span className="text-sm">加载中...</span>
      </div>
    </div>
  )
}

function AppContent() {
  const [authState, setAuthState] = useState<'loading' | 'authenticated' | 'unauthenticated'>('loading')

  useEffect(() => {
    // 本地访问直接跳过登录（后端中间件对本地请求免认证）
    const host = window.location.hostname
    if (host === 'localhost' || host === '127.0.0.1' || host === '::1') {
      setAuthState('authenticated')
      return
    }
    checkSession().then(valid => {
      setAuthState(valid ? 'authenticated' : 'unauthenticated')
    }).catch(() => {
      setAuthState('unauthenticated')
    })
  }, [])

  if (authState === 'loading') {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--bg-base)]">
        <div className="flex flex-col items-center gap-3 text-[var(--text-muted)]">
          <div className="w-8 h-8 border-2 border-blue-500/30 border-t-blue-500 rounded-full animate-spin" />
          <span className="text-sm">PxeGo</span>
        </div>
      </div>
    )
  }

  if (authState === 'unauthenticated') {
    return <LoginPage onLogin={() => setAuthState('authenticated')} />
  }

  return (
    <AppShell>
      <Suspense fallback={<LoadingFallback />}>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/hosts" element={<Hosts />} />
          <Route path="/hosts/:id" element={<HostDetail />} />
          <Route path="/profiles" element={<Profiles />} />
          <Route path="/files" element={<Files />} />
          <Route path="/netboot-catalog" element={<NetbootCatalog />} />
          <Route path="/netboot/answer-templates" element={<AnswerTemplates />} />
          <Route path="/dns/records" element={<DNSRecords />} />
          <Route path="/netboot/access-control" element={<AccessControl />} />
          <Route path="/events" element={<Events />} />
          <Route path="/logs" element={<Logs />} />
          <Route path="/services" element={<Services />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Suspense>
    </AppShell>
  )
}

function App() {
  return (
    <BrowserRouter>
      <ToastProvider>
        <AppContent />
      </ToastProvider>
    </BrowserRouter>
  )
}

export default App
