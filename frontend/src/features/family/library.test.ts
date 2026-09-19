import { describe, expect, it } from 'vitest'
import { activities, activityDraft, matchesActivity, type ActivityFilters } from './library'
import { habitCalendar } from './calendar'
import { emptyDraft, type FamilyEntry } from '../../domain/family'
describe('activity choices', () => {
  it('combines age, time, setting, energy, budget and materials', () => {
    const filters: ActivityFilters = { minutes: 20, age: 4, place: 'home', energy: 'quiet', budget: 'free', query: 'бумага' }
    expect(activities.filter(a => matchesActivity(a, filters)).map(a => a.id)).toEqual(['thanks', 'plant'])
    expect(activities.filter(a => matchesActivity(a, { ...filters, minutes: 5 }))).toEqual([])
  })
  it('keeps age-specific roles, preparation and reflection when planning', () => {
    const draft = activityDraft(activities[0], '2026-09-19')
    expect(draft.description).toContain(activities[0].question)
    expect(draft.description).toContain(activities[0].younger)
    expect(draft.steps).toHaveLength(3)
    expect(draft.steps.every(s => s.participantId === 0)).toBe(true)
  })
  it('exports a recurring local-time reminder with escaped content', () => {
    const e = { ...emptyDraft('2026-09-19'), id: 4, title: 'Чтение, вместе', reminder: '20:30', weekdays: [1, 3, 5], description: 'Книга\nОбсуждение' } as FamilyEntry
    const ics = habitCalendar(e)
    expect(ics).toContain('DTSTART:20260921T203000')
    expect(ics).toContain('RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR')
    expect(ics).toContain('SUMMARY:Чтение\\, вместе')
    expect(ics).toContain('BEGIN:VALARM')
  })
})
