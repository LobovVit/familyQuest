// @vitest-environment jsdom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import type { FamilyQuestGateway, Runtime } from './ports'
import { RuntimeContext } from './runtime'
import { useFamilyLife } from './useFamilyLife'
import { emptyDraft, type FamilyEntry } from '../domain/family'
afterEach(cleanup)
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(done => { resolve = done }); return { promise, resolve } }
const entry: FamilyEntry = { ...emptyDraft(), id: 1, title: 'Читаем', kind: 'habit', version: 1, authorId: 1, archived: false, approved: false, events: [], createdAt: '', updatedAt: '' }
function wrapper(gateway: Partial<FamilyQuestGateway>) {
  const runtime = { gateway } as Runtime
  return ({ children }: { children: ReactNode }) => <RuntimeContext.Provider value={runtime}>{children}</RuntimeContext.Provider>
}
describe('family data lifecycle', () => {
  it('does not overwrite a saved card with an older load response', async () => {
    const pending = deferred<FamilyEntry[]>()
    const { result } = renderHook(useFamilyLife, { wrapper: wrapper({ familyEntries: () => pending.promise, createFamily: async () => entry }) })
    await act(async () => { await result.current.save('habit', emptyDraft()) })
    await act(async () => pending.resolve([]))
    expect(result.current.entries).toEqual([entry])
  })
  it('ignores the first StrictMode load when it finishes after the second', async () => {
    const first = deferred<FamilyEntry[]>()
    const familyEntries = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue([entry])
    const { result } = renderHook(useFamilyLife, { wrapper: wrapper({ familyEntries }), reactStrictMode: true })
    await waitFor(() => expect(result.current.entries).toEqual([entry]))
    await act(async () => first.resolve([]))
    expect(result.current.entries).toEqual([entry])
  })
  it('keeps loading until the latest StrictMode request finishes', async () => {
    const first = deferred<FamilyEntry[]>(); const second = deferred<FamilyEntry[]>()
    const familyEntries = vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const { result } = renderHook(useFamilyLife, { wrapper: wrapper({ familyEntries }), reactStrictMode: true })
    await act(async () => first.resolve([]))
    expect(result.current.loading).toBe(true)
    await act(async () => second.resolve([entry]))
    expect(result.current.loading).toBe(false); expect(result.current.entries).toEqual([entry])
  })
  it('blocks repeated clicks while a mutation is pending', async () => {
    const saved = deferred<FamilyEntry>(); const createFamily = vi.fn().mockReturnValue(saved.promise)
    const { result } = renderHook(useFamilyLife, { wrapper: wrapper({ familyEntries: async () => [], createFamily }) })
    await waitFor(() => expect(result.current.loading).toBe(false))
    let one!: Promise<boolean>; let two!: Promise<boolean>
    act(() => { one = result.current.save('habit', emptyDraft()); two = result.current.save('habit', emptyDraft()) })
    await act(async () => saved.resolve(entry))
    expect(await one).toBe(true); expect(await two).toBe(false); expect(createFamily).toHaveBeenCalledTimes(1)
  })
  it('refreshes a conflicting card and does not claim the action succeeded', async () => {
    const latest = { ...entry, version: 2 }
    const familyEntries = vi.fn().mockResolvedValueOnce([entry]).mockResolvedValue([latest])
    const { result } = renderHook(useFamilyLife, { wrapper: wrapper({ familyEntries, familyAction: async () => { throw Object.assign(new Error('Conflict'), { status: 409 }) } }) })
    await waitFor(() => expect(result.current.loading).toBe(false))
    let ok = true
    await act(async () => { ok = await result.current.act(entry, { action: 'checkin' }) })
    expect(ok).toBe(false); expect(result.current.entries).toEqual([latest]); expect(result.current.error).toContain('Карточка уже изменилась')
  })
})
