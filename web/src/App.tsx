import { BrowserRouter, Routes, Route } from 'react-router-dom'
import './i18n'

function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-gray-950 text-gray-100">
        <Routes>
          <Route path="/" element={<div className="p-8 text-center text-xl">PxeGo</div>} />
        </Routes>
      </div>
    </BrowserRouter>
  )
}

export default App
