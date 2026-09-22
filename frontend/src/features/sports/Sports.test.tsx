// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { Sports } from './Sports'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { emptyDraft, type FamilyEntry } from '../../domain/family'
afterEach(cleanup)
const child = { id: 2, name: 'Ребёнок', role: 'child' as const, active: true }
const parent = { id: 1, name: 'Родитель', role: 'parent' as const, active: true }
const entry: FamilyEntry = { ...emptyDraft('2026-09-20'), participantIds: [2], title: 'Бег', kind: 'sport', sport: { activity: 'Бег', minutes: 30, distanceKm: 5, exercises: [] }, id: 7, version: 1, authorId: 1, archived: false, approved: false, events: [], createdAt: '', updatedAt: '' }
it('child edits own parent-created entry and sends a clean draft', async () => {
  const editFamily = vi.fn(async () => ({ ...entry, version: 2 }))
  const runtime = { gateway: { familyEntries: async () => [entry], editFamily } } as unknown as Runtime
  render(<RuntimeContext.Provider value={runtime}><Sports current={child} participants={[parent, child]} date="2026-09-22" /></RuntimeContext.Provider>)
  fireEvent.click(await screen.findByRole('button', { name: 'Редактировать' }))
  const form = screen.getByRole('heading', { name: 'Запись занятия' }).parentElement!
  expect(within(form).getByLabelText('Участник').querySelectorAll('option')).toHaveLength(1)
  fireEvent.change(within(form).getByLabelText('Длительность, мин'), { target: { value: '40' } })
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить занятие' }))
  await waitFor(() => expect(editFamily).toHaveBeenCalled())
  expect(editFamily.mock.calls[0]).toEqual([7, 1, expect.objectContaining({ sport: expect.objectContaining({ minutes: 40 }) })])
  expect((editFamily.mock.calls as unknown[][])[0][2]).not.toHaveProperty('id')
})
it('preserves unsaved values after a conflict and offers reloading', async () => {
  const familyEntries = vi.fn().mockResolvedValueOnce([entry]).mockResolvedValue([{ ...entry, version: 2 }])
  const runtime = { gateway: { familyEntries, editFamily: async () => { throw Object.assign(new Error('Conflict'), { status: 409 }) } } } as unknown as Runtime
  render(<RuntimeContext.Provider value={runtime}><Sports current={child} participants={[parent, child]} date="2026-09-22" /></RuntimeContext.Provider>)
  fireEvent.click(await screen.findByRole('button', { name: 'Редактировать' }))
  fireEvent.change(screen.getByLabelText('Длительность, мин'), { target: { value: '45' } })
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить занятие' }))
  await screen.findByRole('button', { name: 'Загрузить актуальную версию' })
  expect((screen.getByLabelText('Длительность, мин') as HTMLInputElement).value).toBe('45')
  fireEvent.click(screen.getByRole('button', { name: 'Загрузить актуальную версию' }))
  expect((screen.getByLabelText('Длительность, мин') as HTMLInputElement).value).toBe('30')
})
