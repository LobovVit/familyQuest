import type { FamilyQuestGateway, SessionStore } from './ports'

interface SessionLifecycleStore extends SessionStore {
 getSessionGeneration(): number
 saveSession(value: Parameters<SessionStore['saveSession']>[0], broadcast?: boolean): void
}

// Orchestrates restoration without depending on HTTP, browser storage or React.
export function createSessionLifecycle(gateway: Pick<FamilyQuestGateway, 'restoreSession'>, session: SessionLifecycleStore) {
 let startup: Promise<void> | undefined
 async function refresh() {
  const generation = session.getSessionGeneration()
  try {
   const restored = await gateway.restoreSession()
   if (generation !== session.getSessionGeneration()) return
   if (restored) session.saveSession(restored, false)
   else session.clearSession()
  } catch (error) {
   if (generation !== session.getSessionGeneration()) return
   throw error
  }
 }
 function initialize() {
  return startup ??= refresh().catch(error => { startup = undefined; throw error })
 }
 return { initialize, refresh }
}
