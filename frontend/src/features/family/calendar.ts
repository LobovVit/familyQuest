import type { FamilyEntry } from '../../domain/family'
const escape = (text: string) => text.replace(/\\/g, '\\\\').replace(/\r?\n/g, '\\n').replace(/,/g, '\\,').replace(/;/g, '\\;')
export function habitCalendar(entry: FamilyEntry): string {
  const days = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']
  const first = new Date(`${entry.date}T12:00:00`)
  for (let offset = 0; offset < 7 && !entry.weekdays.includes(first.getDay()); offset++) first.setDate(first.getDate() + 1)
  const date = `${first.getFullYear()}${String(first.getMonth() + 1).padStart(2, '0')}${String(first.getDate()).padStart(2, '0')}`
  const time = (entry.reminder || '18:00').replace(':', '')
  return ['BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//FamilyQuest//Family habits//RU', 'BEGIN:VEVENT', `UID:familyquest-habit-${entry.id}@familyquest.local`, `DTSTAMP:${new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d{3}Z/, 'Z')}`, `DTSTART:${date}T${time}00`, 'DURATION:PT10M', `RRULE:FREQ=WEEKLY;BYDAY=${entry.weekdays.map(d => days[d]).join(',')}`, `SUMMARY:${escape(entry.title)}`, `DESCRIPTION:${escape(entry.description + '\nОблегчённый вариант: ' + entry.easyVersion)}`, 'BEGIN:VALARM', 'TRIGGER:PT0M', 'ACTION:DISPLAY', `DESCRIPTION:${escape(entry.title)}`, 'END:VALARM', 'END:VEVENT', 'END:VCALENDAR', ''].join('\r\n')
}
export function downloadCalendar(entry: FamilyEntry) {
  const url = URL.createObjectURL(new Blob([habitCalendar(entry)], { type: 'text/calendar;charset=utf-8' }))
  const link = document.createElement('a'); link.href = url; link.download = `familyquest-habit-${entry.id}.ics`; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
}
