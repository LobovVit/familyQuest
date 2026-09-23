import { SessionStartup } from './features/session/SessionStartup'
import { createSessionLifecycle } from './application/sessionLifecycle'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { RuntimeContext } from './application/runtime'
import { gateway } from './infrastructure/gateway'
import * as session from './infrastructure/session'
import { downloadBackup, restoreBackup } from './infrastructure/backup'

const runtime = { gateway, session, downloadBackup, restoreBackup }

const lifecycle = createSessionLifecycle(gateway, session)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RuntimeContext.Provider value={runtime}><SessionStartup initialize={lifecycle.initialize} refresh={lifecycle.refresh}><App /></SessionStartup></RuntimeContext.Provider>
  </StrictMode>,
)
