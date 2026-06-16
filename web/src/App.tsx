import { Suspense, lazy } from 'react'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AppShell } from './components/layout/AppShell'
import { ToastProvider } from './components/ui/Toast'
import './i18n'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const Hosts = lazy(() => import('./pages/Hosts'))
const HostDetail = lazy(() => import('./pages/HostDetail'))
const Profiles = lazy(() => import('./pages/Profiles'))
const Files = lazy(() => import('./pages/Files'))
const Events = lazy(() => import('./pages/Events'))
const Settings = lazy(() => import('./pages/Settings'))

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

function App() {
  return (
    <BrowserRouter>
      <ToastProvider>
        <AppShell>
          <Suspense fallback={<LoadingFallback />}>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/hosts" element={<Hosts />} />
              <Route path="/hosts/:id" element={<HostDetail />} />
              <Route path="/profiles" element={<Profiles />} />
              <Route path="/files" element={<Files />} />
              <Route path="/events" element={<Events />} />
              <Route path="/settings" element={<Settings />} />
            </Routes>
          </Suspense>
        </AppShell>
      </ToastProvider>
    </BrowserRouter>
  )
}

export default App
