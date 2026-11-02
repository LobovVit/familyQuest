import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './apiClient'
import { ConflictError } from '../application/errors'

describe('api client errors', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('extracts the API error message from JSON', async () => {
    vi.stubGlobal('sessionStorage', { getItem: vi.fn().mockReturnValue(null), removeItem: vi.fn(), setItem: vi.fn() })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'Неверный PIN' }), { status: 403 })))
    await expect(api('/api/session')).rejects.toThrow('Неверный PIN')
  })
  it('translates an HTTP conflict into the application concurrency contract', async () => {
    vi.stubGlobal('sessionStorage', { getItem: vi.fn().mockReturnValue(null), removeItem: vi.fn(), setItem: vi.fn() })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'Карточка изменена' }), { status: 409 })))
    await expect(api('/api/family/1', { method: 'PUT', body: '{}' })).rejects.toBeInstanceOf(ConflictError)
  })
})

it('keeps the current profile when a Telegram profile-switch PIN fails', async () => {
 const values = new Map<string,string>([['familyquest.session.token','current-profile']])
 vi.stubGlobal('sessionStorage',{getItem:(key:string)=>values.get(key)??null,removeItem:(key:string)=>values.delete(key),setItem:(key:string,value:string)=>values.set(key,value)})
 vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(JSON.stringify({error:'Неверный PIN'}),{status:401})))
 try {
  await expect(api('/api/telegram/profile',{method:'POST',body:'{"participantId":2,"pin":"000000"}'})).rejects.toThrow('Неверный PIN')
  expect(values.get('familyquest.session.token')).toBe('current-profile')
 } finally {vi.unstubAllGlobals()}
})
