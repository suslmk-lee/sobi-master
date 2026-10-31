import React from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { AppearanceProvider, initAppearance } from './appearance/AppearanceContext'
import CursorTrail from './components/effects/CursorTrail'
import CursorPointer from './components/effects/CursorPointer'
import CardTilt from './components/effects/CardTilt'
import App from './App'

// 첫 렌더 전에: 저장된 테마/변수를 <html> 에 바로 적용 → 깜빡임 방지
initAppearance()

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppearanceProvider>
      <CursorTrail />
      <CursorPointer />
      <CardTilt />
      <App />
    </AppearanceProvider>
  </React.StrictMode>
)
