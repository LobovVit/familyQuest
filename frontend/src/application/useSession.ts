import { useCallback, useState } from 'react'
import type { LoginResponse, Participant } from '../domain/models'
import { clearSession, getParticipant, saveSession } from '../infrastructure/session'

export function useSession() {
  const [participant, setParticipant] = useState<Participant | null>(() => getParticipant())
  const login = useCallback((session: LoginResponse) => { saveSession(session); setParticipant(session.participant) }, [])
  const logout = useCallback(() => { clearSession(); setParticipant(null) }, [])
  return { participant, login, logout }
}
