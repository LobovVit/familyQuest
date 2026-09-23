import { useCallback, useSyncExternalStore } from 'react'
import type { LoginResponse } from '../domain/models'
import { useRuntime } from './runtime'

export function useSession() {
 const { session, gateway } = useRuntime()
 const participant = useSyncExternalStore(session.subscribe, session.getParticipant, () => null)
 const login = useCallback((value: LoginResponse) => session.saveSession(value), [session])
 const logout = useCallback(async () => { await gateway.logout(); session.clearSession(true) }, [session, gateway])
 return { participant, login, logout }
}
