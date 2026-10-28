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
