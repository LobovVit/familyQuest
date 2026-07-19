import { useCallback, useEffect, useMemo, useState } from 'react'
import './App.css'

const API_URL = import.meta.env.VITE_API_URL ?? ''

type Participant = {
  id: number
  name: string
  role: 'parent' | 'child'
}

type Chore = {
  id: number
  title: string
  description: string
  schedule: Schedule
  timeWindow: TimeWindow
  baseValue: number
}

type Assignment = {
  id: number
  choreId: number
  participantId: number
  choreTitle: string
  personName: string
  schedule: Schedule
  timeWindow: TimeWindow
  baseValue: number
}

type Task = {
  id: number
  assignmentId: number
  participantId: number
  choreTitle: string
  personName: string
  dueDate: string
  timeWindow: TimeWindow
  status: 'pending' | 'completed' | 'needs_work' | 'confirmed'
  averageRating: number
  reward: number
}

type LeaderboardEntry = {
  participantId: number
  name: string
  tasksDone: number
  reward: number
  averageRating: number
}

type Schedule = 'once' | 'daily' | 'daily_windowed' | 'weekly' | 'monthly'
type TimeWindow = '' | 'morning' | 'day' | 'evening'

const scheduleLabels: Record<Schedule, string> = {
  once: 'Разово',
  daily: 'Ежедневно',
  daily_windowed: 'Ежедневно с окном',
  weekly: 'Еженедельно',
  monthly: 'Ежемесячно',
}

const windowLabels: Record<TimeWindow, string> = {
  '': 'Без окна',
  morning: 'Утро',
  day: 'День',
  evening: 'Вечер',
}

const roleLabels: Record<Participant['role'], string> = {
  parent: 'Взрослый',
  child: 'Дошкольник',
}

function App() {
  const [selectedDate, setSelectedDate] = useState(() => new Date().toISOString().slice(0, 10))
  const [participants, setParticipants] = useState<Participant[]>([])
  const [chores, setChores] = useState<Chore[]>([])
  const [assignments, setAssignments] = useState<Assignment[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([])
  const [selectedPerson, setSelectedPerson] = useState<number | 'all'>('all')
  const [period, setPeriod] = useState<'week' | 'month'>('week')
  const [busyTask, setBusyTask] = useState<number | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')

  const [newChore, setNewChore] = useState({
    title: '',
    description: '',
    schedule: 'daily' as Schedule,
    timeWindow: '' as TimeWindow,
    baseValue: 50,
  })
  const [newAssignment, setNewAssignment] = useState({
    choreId: 0,
    participantId: 0,
  })

  const loadData = useCallback(async () => {
    setError('')
    try {
      const [participantsData, choresData, assignmentsData, tasksData, leaderboardData] =
        await Promise.all([
          api<Participant[]>('/api/participants'),
          api<Chore[]>('/api/chores'),
          api<Assignment[]>('/api/assignments'),
          api<Task[]>(`/api/tasks?date=${selectedDate}`),
          api<LeaderboardEntry[]>(`/api/leaderboard?period=${period}&date=${selectedDate}`),
        ])

      setParticipants(participantsData)
      setChores(choresData)
      setAssignments(assignmentsData)
      setTasks(tasksData)
      setLeaderboard(leaderboardData)
      setNewAssignment((current) => ({
        choreId: current.choreId || choresData[0]?.id || 0,
        participantId: current.participantId || participantsData[0]?.id || 0,
      }))
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Не удалось загрузить данные')
    } finally {
      setIsLoading(false)
    }
  }, [period, selectedDate])

  useEffect(() => {
    void loadData()
  }, [loadData])

  const filteredTasks = useMemo(() => {
    if (selectedPerson === 'all') {
      return tasks
    }
    return tasks.filter((task) => task.participantId === selectedPerson)
  }, [selectedPerson, tasks])

  const totals = useMemo(() => {
    const confirmed = tasks.filter((task) => task.status === 'confirmed')
    const completed = tasks.filter((task) => task.status !== 'pending')
    return {
      total: tasks.length,
      completed: completed.length,
      confirmed: confirmed.length,
      reward: confirmed.reduce((sum, task) => sum + task.reward, 0),
    }
  }, [tasks])

  const maxTasks = useMemo(() => tasks.filter((task) => task.personName === 'Макс'), [tasks])
  const maxCompleted = maxTasks.filter((task) => task.status !== 'pending').length
  const maxProgress = maxTasks.length === 0 ? 0 : Math.round((maxCompleted / maxTasks.length) * 100)

  async function createChore(event: React.FormEvent) {
    event.preventDefault()
    if (!newChore.title.trim()) {
      return
    }
    setError('')
    try {
      await api('/api/chores', {
        method: 'POST',
        body: JSON.stringify(newChore),
      })
      setNewChore({
        title: '',
        description: '',
        schedule: 'daily',
        timeWindow: '',
        baseValue: 50,
      })
      await loadData()
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : 'Не удалось добавить обязанность')
    }
  }

  async function createAssignment(event: React.FormEvent) {
    event.preventDefault()
    if (!newAssignment.choreId || !newAssignment.participantId) {
      return
    }
    setError('')
    try {
      await api('/api/assignments', {
        method: 'POST',
        body: JSON.stringify(newAssignment),
      })
      await loadData()
    } catch (assignmentError) {
      setError(assignmentError instanceof Error ? assignmentError.message : 'Не удалось назначить обязанность')
    }
  }

  async function completeTask(task: Task) {
    setBusyTask(task.id)
    setError('')
    try {
      await api(`/api/tasks/${task.id}/complete`, {
        method: 'POST',
        body: JSON.stringify({ participantId: task.participantId }),
      })
      await loadData()
    } catch (completeError) {
      setError(completeError instanceof Error ? completeError.message : 'Не удалось отметить задачу')
    } finally {
      setBusyTask(null)
    }
  }

  async function confirmTask(task: Task, rating: number) {
    const parent = participants.find((participant) => participant.role === 'parent')
    if (!parent) {
      return
    }
    setBusyTask(task.id)
    setError('')
    try {
      await api(`/api/tasks/${task.id}/confirm`, {
        method: 'POST',
        body: JSON.stringify({ participantId: parent.id, rating, comment: '' }),
      })
      await loadData()
    } catch (confirmError) {
      setError(confirmError instanceof Error ? confirmError.message : 'Не удалось поставить оценку')
    } finally {
      setBusyTask(null)
    }
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">FamilyQuest</p>
          <h1>Семейный план дня</h1>
          <p className="topbar-copy">Маленькие дела Макса и взрослые семейные задачи собираются в один спокойный ритм.</p>
        </div>
        <div className="date-card">
          <span>Дата плана</span>
          <input type="date" value={selectedDate} onChange={(event) => setSelectedDate(event.target.value)} />
        </div>
      </header>

      {error && <p className="notice">{error}</p>}

      <section className="family-board" aria-label="Семья">
        <button className={`person-card all-card ${selectedPerson === 'all' ? 'active' : ''}`} onClick={() => setSelectedPerson('all')}>
          <span className="avatar">FQ</span>
          <strong>Вся семья</strong>
          <small>{tasks.length} дел на день</small>
        </button>
        {participants.map((person) => {
          const personTasks = tasks.filter((task) => task.participantId === person.id)
          const done = personTasks.filter((task) => task.status !== 'pending').length

          return (
            <button
              className={`person-card ${selectedPerson === person.id ? 'active' : ''}`}
              key={person.id}
              onClick={() => setSelectedPerson(person.id)}
            >
              <span className={`avatar ${person.role}`}>{person.name.slice(0, 1)}</span>
              <strong>{person.name}</strong>
              <small>
                {roleLabels[person.role]} · {done}/{personTasks.length}
              </small>
            </button>
          )
        })}
      </section>

      <section className="summary-grid">
        <Metric label="На сегодня" value={totals.total} />
        <Metric label="Отмечено" value={totals.completed} />
        <Metric label="Подтверждено" value={totals.confirmed} />
        <Metric label="Начислено" value={`${Math.round(totals.reward)} уе`} />
      </section>

      <section className="max-progress">
        <div>
          <p className="eyebrow">Фокус для Макса</p>
          <h2>{maxCompleted}/{maxTasks.length} детских дел отмечено</h2>
        </div>
        <div className="progress-track" aria-label={`Прогресс Макса ${maxProgress}%`}>
          <span style={{ width: `${maxProgress}%` }} />
        </div>
      </section>

      <section className="workspace">
        <div className="task-panel">
          <div className="section-heading">
            <div>
              <p className="eyebrow">{formatDate(selectedDate)}</p>
              <h2>Задачи дня</h2>
            </div>
          </div>

          <div className="task-list">
            {isLoading && <p className="empty">Загружаю семейный план...</p>}
            {!isLoading && filteredTasks.length === 0 && <p className="empty">Назначьте обязанности, и здесь появится план дня.</p>}
            {filteredTasks.map((task) => (
              <article className="task-row" key={task.id}>
                <div className="task-main">
                  <div className="task-title-line">
                    <span className={`status ${task.status}`}>{statusLabel(task.status)}</span>
                    <span className="task-owner">{task.personName}</span>
                  </div>
                  <h3>{task.choreTitle}</h3>
                  <p>{windowLabels[task.timeWindow]} · {taskRewardLabel(assignments, task)}</p>
                </div>
                <div className="task-actions">
                  {task.status === 'pending' && (
                    <button disabled={busyTask === task.id} onClick={() => completeTask(task)}>
                      Сделано
                    </button>
                  )}
                  {task.status === 'completed' && (
                    <div className="rating-buttons" aria-label="Оценка выполнения">
                      {[1, 2, 3, 4, 5].map((rating) => (
                        <button disabled={busyTask === task.id} key={rating} onClick={() => confirmTask(task, rating)}>
                          {rating}
                        </button>
                      ))}
                    </div>
                  )}
                  {task.status === 'confirmed' && <strong>{task.reward.toFixed(0)} уе</strong>}
                </div>
              </article>
            ))}
          </div>
        </div>

        <aside className="side-column">
          <section className="panel">
            <div className="section-heading compact">
              <h2>Рейтинг</h2>
              <div className="segmented">
                <button className={period === 'week' ? 'active' : ''} onClick={() => setPeriod('week')}>
                  Неделя
                </button>
                <button className={period === 'month' ? 'active' : ''} onClick={() => setPeriod('month')}>
                  Месяц
                </button>
              </div>
            </div>
            <ol className="leaderboard">
              {leaderboard.map((entry) => (
                <li key={entry.participantId}>
                  <span>{entry.name}</span>
                  <strong>{entry.reward.toFixed(0)} уе</strong>
                  <small>{entry.tasksDone} дел · {entry.averageRating.toFixed(1)}/5</small>
                </li>
              ))}
            </ol>
          </section>

          <section className="panel">
            <h2>Новое назначение</h2>
            <form className="stack-form" onSubmit={createAssignment}>
              <label>
                Обязанность
                <select
                  value={newAssignment.choreId}
                  onChange={(event) => setNewAssignment({ ...newAssignment, choreId: Number(event.target.value) })}
                >
                  {chores.map((chore) => (
                    <option key={chore.id} value={chore.id}>
                      {chore.title}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Участник
                <select
                  value={newAssignment.participantId}
                  onChange={(event) => setNewAssignment({ ...newAssignment, participantId: Number(event.target.value) })}
                >
                  {participants.map((person) => (
                    <option key={person.id} value={person.id}>
                      {person.name}
                    </option>
                  ))}
                </select>
              </label>
              <button type="submit">Назначить</button>
            </form>
          </section>
        </aside>
      </section>

      <section className="catalog">
        <div className="panel">
          <h2>Справочник обязанностей</h2>
          <div className="chore-grid">
            {chores.map((chore) => (
              <article key={chore.id}>
                <h3>{chore.title}</h3>
                <p>{chore.description}</p>
                <footer>
                  <span>{scheduleLabels[chore.schedule]}</span>
                  <strong>{chore.baseValue} уе</strong>
                </footer>
              </article>
            ))}
          </div>
        </div>

        <div className="panel">
          <h2>Добавить обязанность</h2>
          <form className="stack-form" onSubmit={createChore}>
            <label>
              Название
              <input value={newChore.title} onChange={(event) => setNewChore({ ...newChore, title: event.target.value })} />
            </label>
            <label>
              Описание
              <textarea
                value={newChore.description}
                onChange={(event) => setNewChore({ ...newChore, description: event.target.value })}
              />
            </label>
            <div className="form-row">
              <label>
                Периодичность
                <select
                  value={newChore.schedule}
                  onChange={(event) => setNewChore({ ...newChore, schedule: event.target.value as Schedule })}
                >
                  {Object.entries(scheduleLabels).map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Окно
                <select
                  value={newChore.timeWindow}
                  onChange={(event) => setNewChore({ ...newChore, timeWindow: event.target.value as TimeWindow })}
                >
                  {Object.entries(windowLabels).map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <label>
              Базовая ценность
              <input
                min="1"
                type="number"
                value={newChore.baseValue}
                onChange={(event) => setNewChore({ ...newChore, baseValue: Number(event.target.value) })}
              />
            </label>
            <button type="submit">Добавить</button>
          </form>
        </div>
      </section>
    </main>
  )
}

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!response.ok) {
    throw new Error(`API error ${response.status}`)
  }
  return response.json()
}

function statusLabel(status: Task['status']) {
  switch (status) {
    case 'completed':
      return 'Ждет оценки'
    case 'confirmed':
      return 'Принято'
    case 'needs_work':
      return 'Доработать'
    default:
      return 'Запланировано'
  }
}

function taskBaseValue(assignments: Assignment[], task: Task) {
  return assignments.find((assignment) => assignment.id === task.assignmentId)?.baseValue ?? 0
}

function taskRewardLabel(assignments: Assignment[], task: Task) {
  if (task.averageRating > 0) {
    return `${task.reward.toFixed(0)} уе · оценка ${task.averageRating.toFixed(1)}/5`
  }
  return `${taskBaseValue(assignments, task)} уе за выполнение`
}

function formatDate(value: string) {
  const date = new Date(`${value}T00:00:00`)
  return new Intl.DateTimeFormat('ru-RU', {
    day: 'numeric',
    month: 'long',
    weekday: 'long',
  }).format(date)
}

export default App
