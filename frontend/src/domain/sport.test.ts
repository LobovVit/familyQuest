import { describe, it, expect } from 'vitest'
import { sportProgress } from './sport'
import { emptyDraft, entryDraft, type FamilyEntry } from './family'
const session = (id: number, date: string, weight: number, activity = 'Зал'): FamilyEntry => ({ ...emptyDraft(date), id, kind: 'sport', version: 1, authorId: 1, createdAt: '', updatedAt: '', archived: false, approved: false, events: [], sport: { activity, minutes: 30, distanceKm: 0.1, exercises: [{ name: 'Присед', sets: 3, reps: 10, weightKg: weight }] } })
describe('sport progress', () => {
  it('sorts chronologically, groups Monday weeks, and separates sports', () => {
    const result = sportProgress([session(2, '2026-09-21', 30), session(1, '2026-09-20', 20), session(3, '2026-09-22', 15, 'Гимнастика')])
    expect(result.minutes).toBe(90); expect(result.distance).toBe(0.3)
    expect(result.weeks.map(x => [x.date, x.count])).toEqual([['2026-09-14', 1], ['2026-09-21', 2]])
    expect(result.exercises).toHaveLength(2)
    expect(result.exercises[0].first.weightKg).toBe(20); expect(result.exercises[0].last.weightKg).toBe(30)
  })
  it('retains sports fields in editable drafts without sending entry metadata', () => {
    const e = session(1, '2026-09-22', 20), draft = entryDraft(e)
    expect(draft.sport).toEqual(e.sport); expect(draft).not.toHaveProperty('id'); expect(draft).not.toHaveProperty('version')
  })
})
