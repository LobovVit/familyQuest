import { describe, expect, it } from 'vitest'
import { emptyDraft, gardenMilestones, isDue, localDate, skillStage, type FamilyEntry, type FamilyEvent } from './family'
function entry(patch: Partial<FamilyEntry>): FamilyEntry { return { ...emptyDraft('2026-09-19'), id: 1, version: 1, kind: 'habit', title: 'Читаем', authorId: 1, createdAt: '', updatedAt: '', archived: false, approved: false, events: [], ...patch } }
function event(patch: Partial<FamilyEvent>): FamilyEvent { return { actorId: 1, kind: 'checkin', date: '2026-09-19', step: 0, note: '', easy: false, createdAt: '', ...patch } }
describe('family progress', () => {
  it('counts a shared habit day once and retains archived achievements', () => {
    const habit = entry({ archived: true, events: [event({}), event({ actorId: 2 }), event({ date: '2026-09-21', easy: true })] })
    expect(gardenMilestones([habit])).toHaveLength(2)
  })
  it('requires every adventure step before adding a house', () => {
    const adventure = entry({ kind: 'adventure', steps: [{ title: 'A', participantId: 0 }, { title: 'B', participantId: 0 }], events: [event({ kind: 'step', step: 0 }), event({ kind: 'step', step: 0 })] })
    expect(gardenMilestones([adventure])).toHaveLength(0)
    adventure.events.push(event({ kind: 'step', step: 1 }))
    expect(gardenMilestones([adventure])[0]?.icon).toBe('🏡')
  })
  it('keeps skill stages separate for each member', () => {
    const skill = entry({ kind: 'skill', events: [event({ kind: 'stage', step: 4 }), event({ kind: 'stage', step: 1, actorId: 2 })] })
    expect(skillStage(skill, 2)).toBe(1)
    expect(gardenMilestones([skill])).toHaveLength(1)
  })
  it('respects weekdays, start date, approval and archive in the daily plan', () => {
    const habit = entry({ weekdays: [6], date: '2026-09-19' })
    expect(isDue(habit, '2026-09-19')).toBe(true)
    expect(isDue(habit, '2026-09-12')).toBe(false)
    expect(isDue(habit, '2026-09-20')).toBe(false)
    expect(isDue({ ...habit, archived: true }, '2026-09-19')).toBe(false)
    expect(isDue({ ...habit, kind: 'proposal' }, '2026-09-19')).toBe(false)
    expect(isDue({ ...habit, kind: 'proposal', approved: true }, '2026-09-19')).toBe(true)
  })
  it('uses the local calendar date', () => { expect(localDate(new Date(2026, 8, 19, 0, 5))).toBe('2026-09-19') })
})
