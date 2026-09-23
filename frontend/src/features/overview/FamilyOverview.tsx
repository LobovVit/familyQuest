import type { Participant } from '../../domain/models'
import { useFamilyOverview } from '../../application/useFamilyOverview'
import '../family/family.css'
import './overview.css'

export function FamilyOverview({ date, participants }: { date: string; participants: Participant[] }) {
  const { data, error, retry } = useFamilyOverview(date)
  const names = (ids: number[]) => ids.length ? ids.map(id => participants.find(p => p.id === id)?.name ?? 'Участник').join(', ') : 'Вся семья'
  const fmt = (value: number) => value.toLocaleString('ru-RU', { maximumFractionDigits: 2 })
  return <section className="family-life family-overview" aria-label="Семейный обзор">
    <header className="family-row"><div><p className="eyebrow">Вся семья · режим просмотра</p><h2>Спорт, привычки и приключения</h2><p className="family-muted">Чтобы записать занятие или отметить участие, выберите себя и войдите по PIN.</p></div></header>
    {error ? <div className="notice" role="alert">{error}<button onClick={retry}>Повторить загрузку</button></div> : !data ? <p role="status">Загружаем семейный обзор…</p> : <div className="overview-columns">
      <section className="panel" aria-label="Спорт"><h2>🏃 Спорт</h2><p className="family-muted">С начала недели по выбранный день: {data.weekStart} — {data.date}</p><div className="overview-totals"><strong>{data.sports.length} занятий</strong><span>{fmt(data.sports.reduce((sum, e) => sum + e.minutes, 0))} мин</span><span>{fmt(data.sports.reduce((sum, e) => sum + e.distanceKm, 0))} км</span></div>{!data.sports.length && <p className="empty">За этот период занятия ещё не записаны.</p>}<div className="overview-list">{data.sports.map(e => <article key={e.id}><h3>{e.title}</h3><p>{names(e.participantIds)} · {e.date}</p><p>{e.activity} · {fmt(e.minutes)} мин{e.distanceKm > 0 && ` · ${fmt(e.distanceKm)} км`}</p></article>)}</div></section>
      <section className="panel" aria-label="Привычки"><h2>🌱 Привычки</h2><p className="family-muted">По расписанию на {data.date}</p>{!data.habits.length && <p className="empty">На этот день привычки не запланированы.</p>}<div className="overview-list">{data.habits.map(e => <article key={e.id}><h3>{e.title}</h3><p>{names(e.participantIds)}</p><p className={e.completedIds.length ? 'overview-done' : 'family-muted'}>{e.completedIds.length ? `Отметили: ${names(e.completedIds)}` : 'Пока без отметок'}</p></article>)}</div></section>
      <section className="panel" aria-label="Приключения"><h2>🧭 Приключения</h2><p className="family-muted">Планы и прогресс на {data.date}</p>{!data.adventures.length && <p className="empty">Приключений пока нет. После входа можно выбрать занятие для семьи.</p>}<div className="overview-list">{data.adventures.map(e => <article key={e.id}><h3>{e.title}</h3><p>{names(e.participantIds)} · {e.date}</p><progress aria-label={`${e.title}: выполнено ${e.completedSteps} из ${e.totalSteps} шагов`} max={Math.max(e.totalSteps, 1)} value={e.completedSteps} /><p>{e.completedSteps}/{e.totalSteps} шагов{e.totalSteps > 0 && e.completedSteps === e.totalSteps ? ' · Завершено' : e.date > date ? ' · Запланировано' : ' · В процессе'}</p></article>)}</div></section>
    </div>}
  </section>
}
