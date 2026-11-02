import { TelegramMiniApp } from './features/telegram/TelegramMiniApp'
import { createTelegramGateway, createTelegramHost, isTelegramPage, loadTelegramSDK } from './infrastructure/telegram'
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

async function mount() {
 if (isTelegramPage()) {
  try { await loadTelegramSDK() } catch { /* The shell offers normal web entry. */ }
  const client = createTelegramGateway(), host = createTelegramHost()
  const miniRuntime = {...runtime, gateway: {...gateway, login:client.profile, logout:client.logout}}
  createRoot(document.getElementById('root')!).render(<StrictMode><RuntimeContext.Provider value={miniRuntime}><TelegramMiniApp client={client} host={host}/></RuntimeContext.Provider></StrictMode>)
  return
 }
 createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RuntimeContext.Provider value={runtime}><SessionStartup initialize={lifecycle.initialize} refresh={lifecycle.refresh}><App /></SessionStartup></RuntimeContext.Provider>
  </StrictMode>,
 )
}
void mount()
