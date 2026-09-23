// @vitest-environment jsdom
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { FamilyOverview } from './FamilyOverview'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import type { FamilyOverview as Overview } from '../../domain/family'
afterEach(cleanup)
const data: Overview = { date: '2026-09-23', weekStart: '2026-09-21', sports: [{ id: 1, title: 'Бег', participantIds: [1], date: '2026-09-22', activity: 'Бег', minutes: 30, distanceKm: 5 }], habits: [{ id: 2, title: 'Читаем вместе', participantIds: [1], completedIds: [1] }], adventures: [{ id: 3, title: 'Поход', participantIds: [], date: '2026-09-24', completedSteps: 1, totalSteps: 3 }] }
it('shows sport, habits, adventures without mutation controls or private requests', async () => {
 const familyEntries = vi.fn()
 const runtime = { gateway: { familyOverview: async () => data, familyEntries } } as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><FamilyOverview date={data.date} participants={[{ id: 1, name: 'Саша', role: 'child', active: true }]} /></RuntimeContext.Provider>)
 await screen.findByText('Читаем вместе')
 expect(screen.getByText('Отметили: Саша')).toBeTruthy()
 expect(screen.getByText('30 мин')).toBeTruthy()
 expect(screen.getByRole('progressbar').getAttribute('value')).toBe('1')
 expect(screen.queryAllByRole('button')).toHaveLength(0)
 expect(familyEntries).not.toHaveBeenCalled()
})
it('discards an old date response even under StrictMode', async () => {
 let resolve!: (value: Overview) => void
 const old = new Promise<Overview>(done => { resolve = done })
 const runtime = { gateway: { familyOverview: (date: string) => date === '2026-09-22' ? old : Promise.resolve(data) } } as unknown as Runtime
 const view = (date: string) => <RuntimeContext.Provider value={runtime}><FamilyOverview date={date} participants={[]} /></RuntimeContext.Provider>
 const { rerender } = render(view('2026-09-22'), { reactStrictMode: true })
 rerender(view(data.date))
 await screen.findByText('Читаем вместе')
 await act(async () => resolve({ ...data, habits: [{ id: 99, title: 'Старая дата', participantIds: [], completedIds: [] }] }))
 expect(screen.queryByText('Старая дата')).toBeNull()
 expect(screen.getByText('Читаем вместе')).toBeTruthy()
})
it('offers retry after loading failure', async () => {
 const familyOverview = vi.fn().mockRejectedValueOnce(new Error('Нет связи')).mockResolvedValue(data)
 const runtime = { gateway: { familyOverview } } as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><FamilyOverview date={data.date} participants={[]} /></RuntimeContext.Provider>)
 const retry = await screen.findByRole('button', { name: 'Повторить загрузку' })
 await act(async () => retry.click())
 await screen.findByText('Читаем вместе')
})
