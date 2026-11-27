import type { Assignment, LeaderboardEntry, Participant, Task } from '../../domain/models'
import { taskRewardLabel } from '../shared/formatters'
import type { WorkspaceSection } from '../navigation/sections'

type Props = { participant: Participant; tasks: Task[]; assignments: Assignment[]; summary?: LeaderboardEntry; loading: boolean; loadError: string; onRetry: () => void; busyTask: number | null; reviewCount: number; onComplete: (task: Task) => void; onNavigate: (section: WorkspaceSection) => void }
export function MyDay({ participant, tasks, assignments, summary, loading, loadError, onRetry, busyTask, reviewCount, onComplete, onNavigate }: Props) {
 const own = tasks.filter(task => task.participantId === participant.id)
 const remaining = own.filter(task => task.status === 'pending' || task.status === 'needs_work')
 const done = own.filter(task => task.status === 'completed' || task.status === 'confirmed')
 const waiting = own.filter(task => task.status === 'completed').length
 const percent = own.length ? Math.round(done.length / own.length * 100) : 0
 const child = participant.role === 'child'
 const progress = <div className="day-progress"><span aria-hidden="true">{own.length && !remaining.length ? '🌟' : '🌱'}</span><strong>{loading ? '…' : `${done.length} из ${own.length}`}</strong><span>своих дел отмечено</span><progress aria-label="Мои дела" max={100} value={percent} /><small>{waiting ? `Ждут подтверждения: ${waiting}` : 'Каждый шаг имеет значение'}</small></div>
 return <section className="my-day" aria-label="Мой день">
  <div className={child ? 'day-welcome day-welcome-child' : 'day-welcome'}>
   <div>{!child && <p className="eyebrow">Маленькие шаги, большие открытия</p>}<h1>Привет, {participant.name}!</h1>{!child && <p>{loadError ? 'Не удалось загрузить дела.' : loading ? 'Собираем твой день…' : remaining.length ? 'Начни с одного дела. У тебя всё получится.' : own.length ? 'Все дела отмечены. Можно выбрать занятие по душе!' : 'Сегодня есть время для новых открытий.'}</p>}<button className="day-reward-link" onClick={() => onNavigate('earned')}>{loading ? '…' : `${Math.round(summary?.reward ?? 0)} ⭐ · ${Math.round(summary?.behaviorSmiles ?? 0)} 🙂`}<span>За сегодня →</span></button></div>
   {!child && progress}
  </div>
  {participant.role === 'parent' && reviewCount > 0 && <button className="day-review" onClick={() => onNavigate('day')}>🙌 Дела семьи ждут вашей оценки: {reviewCount}<span>Открыть планер →</span></button>}
  <div className="day-section-heading"><div><h2>{child ? 'Мои дела на сегодня' : 'Мои ближайшие дела'}</h2></div><button onClick={() => onNavigate('day')}>Весь план →</button></div>
  <div className="day-task-grid" aria-busy={loading}>
   {loading ? <p className="day-empty">Загружаю дела…</p> : loadError ? <div className="day-empty"><p role="alert">{loadError}</p><button type="button" onClick={onRetry}>Повторить загрузку дел</button></div> : remaining.length ? remaining.slice(0, 3).map((task, index) => <article className={`day-task ${index === 0 ? 'day-task-next' : ''}`} key={task.id}>
    {(index === 0 || task.status === 'needs_work') && <span className="day-task-step">{task.status === 'needs_work' ? 'Попробуем ещё раз' : 'Начни с этого дела'}</span>}<h3>{task.choreTitle}</h3>{task.choreDescription && <p>{task.choreDescription}</p>}<small>{taskRewardLabel(assignments, task)}</small><button disabled={busyTask === task.id} onClick={() => onComplete(task)}>{busyTask === task.id ? 'Сохраняю…' : '✓ Сделано'}</button>
   </article>) : <div className="day-empty"><span aria-hidden="true">{own.length ? '🎉' : '☀️'}</span><h3>{own.length ? 'Ты справился со своими делами!' : 'На сегодня дел пока нет'}</h3><p>{waiting ? 'Отметки сохранены. Звёзды за дела появятся после подтверждения.' : 'Выбери занятие ниже или загляни в планер.'}</p></div>}
  </div>
  {!loading && !loadError && remaining.length > 3 && <button className="day-more" onClick={() => onNavigate('day')}>Ещё дел в планере: {remaining.length - 3} →</button>}
  {child && progress}
  <div className="day-section-heading"><div><p className="eyebrow">Время для себя и семьи</p><h2>Чем займёмся?</h2></div></div>
  <div className="day-activities">
   {child && <button className="day-activity math" onClick={() => onNavigate('math')}><span aria-hidden="true">🔢</span><strong>Поиграем с числами</strong><small>Решай примеры и открывай новое</small><b>Начать →</b></button>}
   {child && <button className="day-activity reading" onClick={() => onNavigate('reading')}><span aria-hidden="true">📖</span><strong>Почитаем по слогам</strong><small>Фразы и предложения по слогам</small><b>Читать →</b></button>}
   <button className="day-activity sport" onClick={() => onNavigate('sport')}><span aria-hidden="true">🏃</span><strong>Время двигаться</strong><small>Запиши занятие и посмотри свой прогресс</small><b>К спорту →</b></button>
   <button className="day-activity adventure" onClick={() => onNavigate('family')}><span aria-hidden="true">🧭</span><strong>Маленькое приключение</strong><small>Привычки, открытия и время вместе</small><b>Выбрать →</b></button>
  </div>
 </section>
}
