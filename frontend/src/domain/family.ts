import { localDate } from './date'
export { localDate } from './date'
export type FamilyKind = 'adventure' | 'habit' | 'value' | 'skill' | 'proposal' | 'council' | 'memory' | 'sport'
export type FamilyStep = { title: string; participantId: number }
export type SportExercise = { name: string; sets: number; reps: number; weightKg: number }
export type SportSession = { activity: string; minutes: number; distanceKm: number; exercises: SportExercise[] }
export type FamilyDraft = {
  sport?: SportSession
  title: string; description: string; date: string; participantIds: number[]; steps: FamilyStep[]
  weekdays: number[]; reminder: string; easyVersion: string; value: string; agreement: string
  nextActivity: string; photo: string; audio: string
}
export type FamilyEvent = { actorId: number; kind: string; date: string; step: number; note: string; easy: boolean; createdAt: string }
export type FamilyEntry = FamilyDraft & {
  id: number; version: number; kind: FamilyKind; authorId: number; createdAt: string; updatedAt: string
  archived: boolean; approved: boolean; events: FamilyEvent[]
}
export type FamilyAction = { action: string; date?: string; step?: number; note?: string; easy?: boolean }
export const kindLabels: Record<FamilyKind, string> = { adventure: 'Приключения', habit: 'Привычки', value: 'Наши ценности', skill: 'Мастерская навыков', proposal: 'Наши идеи', council: 'Семейный совет', memory: 'Капсула памяти', sport: 'Спорт' }
export const kindIcons: Record<FamilyKind, string> = { adventure: '🧭', habit: '🌱', value: '💛', skill: '🛠️', proposal: '💡', council: '💬', memory: '📷', sport: '🏃' }
export const stageLabels = ['Посмотрел', 'Сделал вместе', 'Попробовал сам', 'Объяснил другому']
export const weekdays = [{ id: 1, label: 'Пн' }, { id: 2, label: 'Вт' }, { id: 3, label: 'Ср' }, { id: 4, label: 'Чт' }, { id: 5, label: 'Пт' }, { id: 6, label: 'Сб' }, { id: 0, label: 'Вс' }]
export function emptyDraft(date = localDate()): FamilyDraft {
  return { title: '', description: '', date, participantIds: [], steps: [], weekdays: [1, 2, 3, 4, 5, 6, 0], reminder: '', easyVersion: '', value: '', agreement: '', nextActivity: '', photo: '', audio: '' }
}
export function entryDraft(entry: FamilyEntry): FamilyDraft {
  const { title, description, date, participantIds, steps, weekdays, reminder, easyVersion, value, agreement, nextActivity, photo, audio, sport } = entry
  return { title, description, date, participantIds: participantIds ?? [], steps: steps ?? [], weekdays: weekdays ?? [], reminder, easyVersion, value, agreement, nextActivity, photo, audio, ...(sport ? { sport } : {}) }
}
export function skillStage(entry: FamilyEntry, actorId: number): number {
  return Math.max(0, ...entry.events.filter(e => e.kind === 'stage' && e.actorId === actorId).map(e => e.step))
}
export function isDue(entry: FamilyEntry, date: string): boolean {
  if (entry.archived) return false
  if (entry.kind === 'habit') return (!entry.date || entry.date <= date) && (entry.weekdays ?? []).includes(new Date(`${date}T12:00:00`).getDay())
  return entry.date === date && ['adventure', 'council', 'proposal'].includes(entry.kind) && (entry.kind !== 'proposal' || entry.approved)
}
// A day of a shared habit counts once, irrespective of family size or repeated clicks.
export function gardenMilestones(entries: FamilyEntry[]): { key: string; icon: string; title: string; date: string }[] {
  return entries.flatMap(e => {
    if (e.kind === 'memory') return [{ key: `memory-${e.id}`, icon: '🌼', title: e.title, date: e.date }]
    if (e.kind === 'council' && e.agreement.trim()) return [{ key: `council-${e.id}`, icon: '🪴', title: e.title, date: e.date }]
    if (['adventure', 'proposal'].includes(e.kind) && (e.steps ?? []).length > 0 && e.steps.every((_, i) => e.events.some(v => v.kind === 'step' && v.step === i))) return [{ key: `adventure-${e.id}`, icon: '🏡', title: e.title, date: e.date }]
    if (e.kind === 'skill') return e.events.filter(v => v.kind === 'stage' && v.step === 4).map(v => ({ key: `skill-${e.id}-${v.actorId}`, icon: '🌳', title: e.title, date: v.date }))
    if (['habit', 'value'].includes(e.kind)) return [...new Set(e.events.filter(v => v.kind === 'checkin').map(v => v.date))].map(date => ({ key: `day-${e.id}-${date}`, icon: e.kind === 'habit' ? '🌱' : '🌷', title: e.title, date }))
    return []
  }).sort((a, b) => b.date.localeCompare(a.date))
}

export type FamilyOverviewCard = { id: number; title: string; participantIds: number[] }
export type FamilyOverview = {
  date: string; weekStart: string
  sports: (FamilyOverviewCard & { date: string; activity: string; minutes: number; distanceKm: number })[]
  habits: (FamilyOverviewCard & { completedIds: number[] })[]
  adventures: (FamilyOverviewCard & { date: string; completedSteps: number; totalSteps: number })[]
}
