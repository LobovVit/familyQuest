// @vitest-environment jsdom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import type { Participant, Task } from '../domain/models'
import type { FamilyQuestGateway, Runtime } from './ports'
import { RuntimeContext } from './runtime'
import { useFamilyQuestData } from './useFamilyQuestData'
import { useSession } from './useSession'
import * as session from '../infrastructure/session'
import { api, download } from '../infrastructure/apiClient'

afterEach(() => { cleanup(); session.clearSession(); vi.unstubAllGlobals() })
const parent: Participant = { id: 1, role: 'parent', active: true, name: 'Parent' }
function deferred<T>() {
 let resolve!: (value: T) => void
 const promise = new Promise<T>(done => { resolve = done })
 return { promise, resolve }
}
function setup(tasks: FamilyQuestGateway['tasks']) {
 const gateway = { participants: async () => [], chores: async () => [], assignments: async () => [], rewards: async () => [], ratings: async () => [], leaderboard: async () => [], tasks } as unknown as FamilyQuestGateway
 const runtime: Runtime = { gateway, session, downloadBackup: async () => {}, restoreBackup: async () => {} }
 return ({ children }: { children: ReactNode }) => <RuntimeContext.Provider value={runtime}>{children}</RuntimeContext.Provider>
}
describe('loading and session lifecycle', () => {
 it('ignores the previous date response when requests finish out of order', async () => {
  session.saveSession({ participant: parent, token: 'old' })
  const first = deferred<Task[]>()
  const wrapper = setup(date => date === '2026-09-18' ? first.promise : Promise.resolve([{ id: 2 } as Task]))
  const { result, rerender } = renderHook(({ date }) => useFamilyQuestData(date, session.getParticipant()), { wrapper, initialProps: { date: '2026-09-18' } })
  const oldRefresh = result.current.refresh
  rerender({ date: '2026-09-19' })
  await waitFor(() => expect(result.current.tasks[0]?.id).toBe(2))
  await act(async () => first.resolve([{ id: 1 } as Task]))
  expect(result.current.tasks[0]?.id).toBe(2)
  await act(async () => oldRefresh())
  expect(result.current.tasks[0]?.id).toBe(2)
 })
 it('does not restore protected data after logout', async () => {
  session.saveSession({ participant: parent, token: 'old' })
  const pending = deferred<Task[]>()
  const { result } = renderHook(() => {
   const auth = useSession()
   return { auth, data: useFamilyQuestData('2026-09-19', auth.participant) }
  }, { wrapper: setup(() => pending.promise) })
  act(() => session.clearSession())
  await act(async () => pending.resolve([{ id: 1 } as Task]))
  expect(result.current.auth.participant).toBeNull()
  expect(result.current.data.tasks).toEqual([])
 })
 it('updates the rendered session on a protected 401', async () => {
  session.saveSession({ participant: parent, token: 'old' })
  const { result } = renderHook(() => useSession(), { wrapper: setup(async () => []) })
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 401 })))
  await act(async () => { await expect(api('/api/chores')).rejects.toThrow() })
  expect(result.current.participant).toBeNull()
 })
 it('keeps a newly established session when an older request returns 401', async () => {
  session.saveSession({ participant: parent, token: 'old' })
  const response = deferred<Response>()
  vi.stubGlobal('fetch', vi.fn().mockReturnValue(response.promise))
  const pending = api('/api/chores')
  session.saveSession({ participant: { ...parent, id: 2 }, token: 'new' })
  response.resolve(new Response('{}', { status: 401 }))
  await expect(pending).rejects.toThrow()
  expect(session.getToken()).toBe('new')
 })
 it('a failed login does not clear the existing session', async () => {
  session.saveSession({ participant: parent, token: 'old' })
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 401 })))
  await expect(api('/api/session')).rejects.toThrow()
  expect(session.getToken()).toBe('old')
 })
 it('backup download also expires the session on 401', async () => {
  session.saveSession({ participant: parent, token: 'old' })
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 401 })))
  await expect(download('/api/backup')).rejects.toThrow()
  expect(session.getParticipant()).toBeNull()
 })
})
