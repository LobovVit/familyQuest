import type { LoginResponse, Participant } from '../domain/models'

const TOKEN_KEY = 'familyquest.session.token'
const PARTICIPANT_KEY = 'familyquest.session.participant'
const listeners = new Set<() => void>()
let cachedValue: string | null = null
let cachedParticipant: Participant | null = null
const notify = () => listeners.forEach(listener => listener())
export function subscribe(listener: () => void): () => void {
 listeners.add(listener)
 return () => { listeners.delete(listener) }
}
export function saveSession(session: LoginResponse): void {
 sessionStorage.setItem(TOKEN_KEY, session.token)
 sessionStorage.setItem(PARTICIPANT_KEY, JSON.stringify(session.participant))
 notify()
}
export const getToken = (): string | null => sessionStorage.getItem(TOKEN_KEY)
export function getParticipant(): Participant | null {
 const value = getToken() ? sessionStorage.getItem(PARTICIPANT_KEY) : null
 if (value === cachedValue) return cachedParticipant
 cachedValue = value
 cachedParticipant = null
 if (value) {
  try {
   const parsed = JSON.parse(value) as Participant
   if (parsed && Number.isInteger(parsed.id) && parsed.id > 0 && parsed.active === true && typeof parsed.name === 'string' && ['parent', 'child', 'school'].includes(parsed.role)) cachedParticipant = parsed
  } catch { /* Invalid local data represents an anonymous session. */ }
 }
 return cachedParticipant
}
export function clearSession(): void {
 sessionStorage.removeItem(TOKEN_KEY)
 sessionStorage.removeItem(PARTICIPANT_KEY)
 notify()
}
