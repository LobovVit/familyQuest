import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { RuntimeContext } from './application/runtime'
import { gateway } from './infrastructure/gateway'
import * as session from './infrastructure/session'
import { downloadBackup, restoreBackup } from './infrastructure/backup'

const runtime = { gateway, session, downloadBackup, restoreBackup }

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RuntimeContext.Provider value={runtime}><App /></RuntimeContext.Provider>
  </StrictMode>,
)
