import { SessionStartup } from './features/session/SessionStartup'
import { ApiError } from './infrastructure/apiClient'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { RuntimeContext } from './application/runtime'
import { gateway } from './infrastructure/gateway'
import * as session from './infrastructure/session'
import { downloadBackup, restoreBackup } from './infrastructure/backup'

const runtime = { gateway, session, downloadBackup, restoreBackup }

let startup: Promise<void> | undefined
function refreshSession() {
 const generation = session.getSessionGeneration()
 return gateway.restoreSession().then(v=>{if(generation===session.getSessionGeneration())session.saveSession(v,false)}).catch(error=>{
  if(generation!==session.getSessionGeneration())return
  if (error instanceof ApiError && error.status === 401) { session.clearSession(); return }
  throw error
 })
}
function initializeSession() { return startup ??= refreshSession() }

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RuntimeContext.Provider value={runtime}><SessionStartup initialize={initializeSession} refresh={refreshSession}><App /></SessionStartup></RuntimeContext.Provider>
  </StrictMode>,
)
