import type { LoginResponse, Participant } from '../domain/models'

const TOKEN_KEY = 'familyquest.session.token'
const PARTICIPANT_KEY = 'familyquest.session.participant'

export function saveSession(session: LoginResponse): void {
  sessionStorage.setItem(TOKEN_KEY, session.token)
  sessionStorage.setItem(PARTICIPANT_KEY, JSON.stringify(session.participant))
}
export const getToken = (): string | null => sessionStorage.getItem(TOKEN_KEY)
export function getParticipant(): Participant | null {
  const value = sessionStorage.getItem(PARTICIPANT_KEY)
  if (!value) return null
  try { return JSON.parse(value) as Participant } catch { clearSession(); return null }
}
export function clearSession(): void {
  sessionStorage.removeItem(TOKEN_KEY)
  sessionStorage.removeItem(PARTICIPANT_KEY)
}
