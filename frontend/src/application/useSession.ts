import { useCallback, useSyncExternalStore } from 'react'
import type { LoginResponse } from '../domain/models'
import { useRuntime } from './runtime'

export function useSession() {
 const { session } = useRuntime()
 const participant = useSyncExternalStore(session.subscribe, session.getParticipant, () => null)
 const login = useCallback((value: LoginResponse) => session.saveSession(value), [session])
 const logout = useCallback(() => session.clearSession(), [session])
 return { participant, login, logout }
}
