import { clearSession, getToken } from './session'

const API_URL = import.meta.env.VITE_API_URL ?? ''
export class ApiError extends Error {
  readonly status: number
  constructor(message:string, status:number) { super(message); this.status = status }
}
export async function api<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set("X-FamilyQuest", "1")
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(`${API_URL}${path}`, { ...init, headers, credentials:'same-origin' })
  if (!response.ok) {
    if (response.status === 401 && !path.startsWith('/api/session') && token === getToken()) clearSession()
    const text = await response.text()
    let message = text || `HTTP ${response.status}`
    try { const payload = JSON.parse(text) as { error?:unknown }; if (typeof payload.error === 'string') message = payload.error } catch { /* plain-text response */ }
    throw new ApiError(message, response.status)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export async function download(path: string): Promise<Blob> {
  const token = getToken()
  const headers = new Headers(token ? { Authorization: `Bearer ${token}` } : undefined)
  headers.set('X-FamilyQuest','1')
  const response = await fetch(`${API_URL}${path}`, { headers })
  if (!response.ok) {
    if (response.status === 401 && token === getToken()) clearSession()
    throw new ApiError('Не удалось выгрузить данные', response.status)
  }
  return response.blob()
}
