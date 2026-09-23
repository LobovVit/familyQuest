import type { LoginResponse, Participant } from '../domain/models'

const DEVICE_KEY = 'familyquest.session.device'
const CHANGE_KEY = 'familyquest.session.change'
const publish = () => { try { localStorage.setItem(CHANGE_KEY, String(Date.now()) + Math.random()) } catch { /* private storage may be unavailable */ } }
if (typeof window !== 'undefined') window.addEventListener('storage', e => { if (e.key === CHANGE_KEY) { clearSession(); window.location.reload() } })
const TOKEN_KEY = 'familyquest.session.token'
const PARTICIPANT_KEY = 'familyquest.session.participant'
let generation = 0
export const getSessionGeneration = () => generation
const listeners = new Set<() => void>()
let cachedValue: string | null = null
let cachedParticipant: Participant | null = null
const notify = () => listeners.forEach(listener => listener())
export function subscribe(listener: () => void): () => void {
 listeners.add(listener)
 return () => { listeners.delete(listener) }
}
export function saveSession(session: LoginResponse, broadcast = true): void {
 generation++
 sessionStorage.setItem(TOKEN_KEY, session.token)
 if (session.remembered) sessionStorage.setItem(DEVICE_KEY, session.deviceId ?? 'remembered')
 else sessionStorage.removeItem(DEVICE_KEY)
 sessionStorage.setItem(PARTICIPANT_KEY, JSON.stringify(session.participant))
 notify()
 if (broadcast) publish()
}
export const getToken = (): string | null => sessionStorage.getItem(TOKEN_KEY)
export function getParticipant(): Participant | null {
 const value = (getToken() || sessionStorage.getItem(DEVICE_KEY)) ? sessionStorage.getItem(PARTICIPANT_KEY) : null
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
export function clearSession(broadcast = false): void {
 generation++
 sessionStorage.removeItem(DEVICE_KEY)
 sessionStorage.removeItem(TOKEN_KEY)
 sessionStorage.removeItem(PARTICIPANT_KEY)
 notify()
 if (broadcast) publish()
}
