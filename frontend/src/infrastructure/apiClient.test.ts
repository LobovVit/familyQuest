import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './apiClient'

describe('api client errors', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('extracts the API error message from JSON', async () => {
    vi.stubGlobal('sessionStorage', { getItem: vi.fn().mockReturnValue(null), removeItem: vi.fn(), setItem: vi.fn() })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'Неверный PIN' }), { status: 403 })))
    await expect(api('/api/session')).rejects.toThrow('Неверный PIN')
  })
})
