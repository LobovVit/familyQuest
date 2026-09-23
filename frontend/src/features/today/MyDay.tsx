import type { Assignment, LeaderboardEntry, Participant, Task } from '../../domain/models'
import { taskRewardLabel } from '../shared/formatters'
import type { WorkspaceSection } from '../navigation/sections'

type Props = { participant: Participant; tasks: Task[]; assignments: Assignment[]; summary?: LeaderboardEntry; loading: boolean; busyTask: number | null; reviewCount: number; onComplete: (task: Task) => void; onNavigate: (section: WorkspaceSection) => void }
export function MyDay({ participant, tasks, assignments, summary, loading, busyTask, reviewCount, onComplete, onNavigate }: Props) {
 const own = tasks.filter(task => task.participantId === participant.id)
 const remaining = own.filter(task => task.status === 'pending' || task.status === 'needs_work')
 const done = own.filter(task => task.status === 'completed' || task.status === 'confirmed')
 const waiting = own.filter(task => task.status === 'completed').length
 const percent = own.length ? Math.round(done.length / own.length * 100) : 0
 const child = participant.role === 'child'
 return <section className="my-day" aria-label="Мой день">
  <div className="day-welcome">
   <div><p className="eyebrow">Маленькие шаги, большие открытия</p><h1>Привет, {participant.name}!</h1><p>{loading ? 'Собираем твой день…' : remaining.length ? 'Начни с одного дела. У тебя всё получится.' : own.length ? 'Все дела отмечены. Можно выбрать занятие по душе!' : 'Сегодня есть время для новых открытий.'}</p><button className="day-reward-link" onClick={() => onNavigate('earned')}>{loading ? '…' : `${Math.round(summary?.reward ?? 0)} ⭐ · ${Math.round(summary?.behaviorSmiles ?? 0)} 🙂`}<span>За выбранный день →</span></button></div>
   <div className="day-progress"><span aria-hidden="true">{own.length && !remaining.length ? '🌟' : '🌱'}</span><strong>{loading ? '…' : `${done.length} из ${own.length}`}</strong><span>своих дел отмечено</span><progress aria-label="Мои дела" max={100} value={percent} /><small>{waiting ? `Ждут подтверждения: ${waiting}` : 'Каждый шаг имеет значение'}</small></div>
  </div>
  {participant.role === 'parent' && reviewCount > 0 && <button className="day-review" onClick={() => onNavigate('day')}>🙌 Дела семьи ждут вашей оценки: {reviewCount}<span>Открыть планер →</span></button>}
  <div className="day-section-heading"><div><p className="eyebrow">Шаг за шагом</p><h2>Мои ближайшие дела</h2></div><button onClick={() => onNavigate('day')}>Весь план →</button></div>
  <div className="day-task-grid" aria-busy={loading}>
   {loading ? <p className="day-empty">Загружаю дела…</p> : remaining.length ? remaining.slice(0, 3).map((task, index) => <article className={`day-task ${index === 0 ? 'day-task-next' : ''}`} key={task.id}>
    <span className="day-task-step">{task.status === 'needs_work' ? 'Попробуем ещё раз' : index === 0 ? 'Можно начать с этого' : `Следующий шаг · ${index + 1}`}</span><h3>{task.choreTitle}</h3>{task.choreDescription && <p>{task.choreDescription}</p>}<small>{taskRewardLabel(assignments, task)}</small><button disabled={busyTask === task.id} onClick={() => onComplete(task)}>{busyTask === task.id ? 'Сохраняю…' : '✓ Сделано'}</button>
   </article>) : <div className="day-empty"><span aria-hidden="true">{own.length ? '🎉' : '☀️'}</span><h3>{own.length ? 'Ты справился со своими делами!' : 'Пока нет назначенных дел'}</h3><p>{waiting ? 'Отметки сохранены. Звёзды за дела появятся после подтверждения.' : 'Выбери занятие ниже или загляни в планер.'}</p></div>}
  </div>
  {!loading && remaining.length > 3 && <button className="day-more" onClick={() => onNavigate('day')}>Ещё дел в планере: {remaining.length - 3} →</button>}
  <div className="day-section-heading"><div><p className="eyebrow">Время для себя и семьи</p><h2>Чем займёмся?</h2></div></div>
  <div className="day-activities">
   {child && <button className="day-activity math" onClick={() => onNavigate('math')}><span aria-hidden="true">🔢</span><strong>Поиграем с числами</strong><small>Решай примеры и открывай новое</small><b>Начать →</b></button>}
   <button className="day-activity sport" onClick={() => onNavigate('sport')}><span aria-hidden="true">🏃</span><strong>Время двигаться</strong><small>Запиши занятие и посмотри свой прогресс</small><b>К спорту →</b></button>
   <button className="day-activity adventure" onClick={() => onNavigate('family')}><span aria-hidden="true">🧭</span><strong>Маленькое приключение</strong><small>Привычки, открытия и время вместе</small><b>Выбрать →</b></button>
  </div>
 </section>
}
