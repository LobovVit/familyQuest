import type { FamilyEntry, SportExercise } from './family'
import { localDate } from './date'
export function sportProgress(entries: FamilyEntry[]) {
  const weeks = new Map<string, { date: string; minutes: number; count: number }>()
  type Point = SportExercise & { date: string }
  const exercises = new Map<string, { name: string; first: Point; last: Point; best: number }>()
  let minutes = 0, distance = 0
  for (const e of [...entries].sort((a, b) => a.date.localeCompare(b.date) || a.id - b.id)) {
    if (e.kind !== 'sport' || !e.sport) continue
    minutes += e.sport.minutes; distance += e.sport.distanceKm
    const day = new Date(`${e.date}T12:00:00`)
    day.setDate(day.getDate() - (day.getDay() + 6) % 7)
    const key = localDate(day), week = weeks.get(key) ?? { date: key, minutes: 0, count: 0 }
    week.minutes += e.sport.minutes; week.count++; weeks.set(key, week)
    for (const x of e.sport.exercises ?? []) {
      const key = `${e.sport.activity.trim().toLocaleLowerCase('ru')}|${x.name.trim().toLocaleLowerCase('ru')}`
      const point = { ...x, date: e.date }, old = exercises.get(key)
      exercises.set(key, { name: `${x.name} · ${e.sport.activity}`, first: old?.first ?? point, last: point, best: Math.max(old?.best ?? 0, x.weightKg) })
    }
  }
  return { minutes: Math.round(minutes * 100) / 100, distance: Math.round(distance * 100) / 100, weeks: [...weeks.values()], exercises: [...exercises.values()] }
}
