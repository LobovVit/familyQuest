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
  benefitType: BenefitType
  executionMode: ExecutionMode
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
  benefitType: BenefitType
  executionMode: ExecutionMode
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
  benefitType: BenefitType
  executionMode: ExecutionMode
  status: 'pending' | 'completed' | 'needs_work' | 'confirmed'
  averageRating: number
  reward: number
}

type LeaderboardEntry = {
  participantId: number
  name: string
  tasksDone: number
  tasksAssigned: number
  reward: number
  averageRating: number
  behaviorRating: number
  behaviorCount: number
}

type PinPrompt = {
  participant: Participant
  pin: string
}

type Schedule = 'once' | 'daily' | 'weekly' | 'monthly'
type TimeWindow = '' | 'morning' | 'day' | 'evening'
type BenefitType = 'self' | 'family' | 'care' | 'home'
type ExecutionMode = 'assigned' | 'together' | 'adult_child' | 'anyone'
type ActiveTab = 'day' | 'week' | 'month' | 'catalog' | 'users'

const scheduleLabels: Record<Schedule, string> = {
  once: 'Разово',
  daily: 'Ежедневно',
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

const benefitLabels: Record<BenefitType, string> = {
  self: 'Для себя',
  family: 'Для семьи',
  care: 'Забота',
  home: 'Дом',
}

const executionLabels: Record<ExecutionMode, string> = {
  assigned: 'Закреплено',
  together: 'Можно вместе',
  adult_child: 'Взрослый + ребенок',
  anyone: 'Кто угодно',
}

const tabs: Array<{ id: ActiveTab; label: string }> = [
  { id: 'day', label: 'План дня' },
  { id: 'week', label: 'План недели' },
  { id: 'month', label: 'Месячный рейтинг' },
  { id: 'catalog', label: 'Справочник обязанностей' },
  { id: 'users', label: 'Настройки пользователей' },
]

function App() {
  const [selectedDate, setSelectedDate] = useState(() => new Date().toISOString().slice(0, 10))
  const [participants, setParticipants] = useState<Participant[]>([])
  const [chores, setChores] = useState<Chore[]>([])
  const [assignments, setAssignments] = useState<Assignment[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [dayLeaderboard, setDayLeaderboard] = useState<LeaderboardEntry[]>([])
  const [weekLeaderboard, setWeekLeaderboard] = useState<LeaderboardEntry[]>([])
  const [monthLeaderboard, setMonthLeaderboard] = useState<LeaderboardEntry[]>([])
  const [selectedPerson, setSelectedPerson] = useState<number | 'all'>('all')
  const [activeTab, setActiveTab] = useState<ActiveTab>('day')
  const [busyTask, setBusyTask] = useState<number | null>(null)
  const [busyBehavior, setBusyBehavior] = useState<number | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')
  const [currentParticipant, setCurrentParticipant] = useState<Participant | null>(null)
  const [pinPrompt, setPinPrompt] = useState<PinPrompt | null>(null)
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false)
  const [isCheckingPin, setIsCheckingPin] = useState(false)

  const [newChore, setNewChore] = useState({
    title: '',
    description: '',
    schedule: 'daily' as Schedule,
    timeWindow: '' as TimeWindow,
    benefitType: 'self' as BenefitType,
    executionMode: 'assigned' as ExecutionMode,
    baseValue: 50,
  })
  const [newAssignment, setNewAssignment] = useState({
    choreId: 0,
    participantId: 0,
  })

  const loadData = useCallback(async () => {
    setError('')
    try {
      const [participantsData, choresData, assignmentsData, tasksData, dayLeaderboardData, weekLeaderboardData, monthLeaderboardData] =
        await Promise.all([
          api<Participant[]>('/api/participants'),
          api<Chore[]>('/api/chores'),
          api<Assignment[]>('/api/assignments'),
          api<Task[]>(`/api/tasks?date=${selectedDate}`),
          api<LeaderboardEntry[]>(`/api/leaderboard?period=day&date=${selectedDate}`),
          api<LeaderboardEntry[]>(`/api/leaderboard?period=week&date=${selectedDate}`),
          api<LeaderboardEntry[]>(`/api/leaderboard?period=month&date=${selectedDate}`),
        ])

      setParticipants(participantsData)
      setChores(choresData)
      setAssignments(assignmentsData)
      setTasks(tasksData)
      setDayLeaderboard(dayLeaderboardData)
      setWeekLeaderboard(weekLeaderboardData)
      setMonthLeaderboard(monthLeaderboardData)
      setNewAssignment((current) => ({
        choreId: current.choreId || choresData[0]?.id || 0,
        participantId: current.participantId || participantsData[0]?.id || 0,
      }))
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Не удалось загрузить данные')
    } finally {
      setIsLoading(false)
    }
  }, [selectedDate])

  useEffect(() => {
    void loadData()
  }, [loadData])

  const filteredTasks = useMemo(() => {
    if (currentParticipant) {
      return tasks.filter((task) => task.participantId === currentParticipant.id)
    }
    if (selectedPerson === 'all') {
      return tasks
    }
    return tasks.filter((task) => task.participantId === selectedPerson)
  }, [currentParticipant, selectedPerson, tasks])

  const totals = useMemo(() => {
    const confirmed = filteredTasks.filter((task) => task.status === 'confirmed')
    const completed = filteredTasks.filter((task) => task.status !== 'pending')
    return {
      total: filteredTasks.length,
      completed: completed.length,
      confirmed: confirmed.length,
      reward: confirmed.reduce((sum, task) => sum + task.reward, 0),
    }
  }, [filteredTasks])

  const maxTasks = useMemo(() => tasks.filter((task) => task.personName === 'Макс'), [tasks])
  const maxCompleted = maxTasks.filter((task) => task.status !== 'pending').length
  const maxProgress = maxTasks.length === 0 ? 0 : Math.round((maxCompleted / maxTasks.length) * 100)

  const tasksForReview = useMemo(() => {
    if (!currentParticipant) {
      return []
    }
    return tasks.filter((task) => task.status === 'completed' && task.participantId !== currentParticipant.id)
  }, [currentParticipant, tasks])

  const weekPlan = useMemo(() => {
    const weekdays = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']
    return weekdays.map((day, index) => {
      const isWeeklyDay = index === 5
      const choresForDay = assignments.filter((assignment) => {
        if (assignment.schedule === 'daily') {
          return true
        }
        return assignment.schedule === 'weekly' && isWeeklyDay
      })

      return {
        day,
        chores: choresForDay,
      }
    })
  }, [assignments])

  function askForParticipant(participant: Participant) {
    if (currentParticipant?.id === participant.id) {
      setIsUserMenuOpen(false)
      return
    }
    setError('')
    setIsUserMenuOpen(false)
    setPinPrompt({ participant, pin: '' })
  }

  function enterViewMode() {
    setCurrentParticipant(null)
    setSelectedPerson('all')
    setError('')
    setIsUserMenuOpen(false)
  }

  async function verifyPin(event: React.FormEvent) {
    event.preventDefault()
    if (!pinPrompt || pinPrompt.pin.length !== 6) {
      setError('PIN должен содержать 6 цифр')
      return
    }
    setIsCheckingPin(true)
    setError('')
    try {
      const participant = await api<Participant>('/api/session', {
        method: 'POST',
        body: JSON.stringify({
          participantId: pinPrompt.participant.id,
          pin: pinPrompt.pin,
        }),
      })
      setCurrentParticipant(participant)
      setSelectedPerson(participant.id)
      setPinPrompt(null)
    } catch (pinError) {
      setError(pinError instanceof Error ? pinError.message : 'Неверный PIN')
    } finally {
      setIsCheckingPin(false)
    }
  }

  function requireCurrentParticipant() {
    if (currentParticipant) {
      return true
    }
    setError('Сначала выберите, кто сейчас на сайте')
    return false
  }

  async function createChore(event: React.FormEvent) {
    event.preventDefault()
    if (!requireCurrentParticipant()) {
      return
    }
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
        benefitType: 'self',
        executionMode: 'assigned',
        baseValue: 50,
      })
      await loadData()
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : 'Не удалось добавить обязанность')
    }
  }

  async function createAssignment(event: React.FormEvent) {
    event.preventDefault()
    if (!requireCurrentParticipant()) {
      return
    }
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
    if (!requireCurrentParticipant()) {
      return
    }
    if (currentParticipant?.id !== task.participantId) {
      setError(`Отметить это дело может ${task.personName}`)
      return
    }
    setBusyTask(task.id)
    setError('')
    try {
      await api(`/api/tasks/${task.id}/complete`, {
        method: 'POST',
        body: JSON.stringify({ participantId: currentParticipant.id }),
      })
      await loadData()
    } catch (completeError) {
      setError(completeError instanceof Error ? completeError.message : 'Не удалось отметить задачу')
    } finally {
      setBusyTask(null)
    }
  }

  async function confirmTask(task: Task, rating: number) {
    if (!requireCurrentParticipant()) {
      return
    }
    const reviewer = currentParticipant
    if (!reviewer) {
      return
    }
    if (reviewer.id === task.participantId) {
      setError('Подтверждать можно дела других участников')
      return
    }
    setBusyTask(task.id)
    setError('')
    try {
      await api(`/api/tasks/${task.id}/confirm`, {
        method: 'POST',
        body: JSON.stringify({ participantId: reviewer.id, rating, comment: '' }),
      })
      await loadData()
    } catch (confirmError) {
      setError(confirmError instanceof Error ? confirmError.message : 'Не удалось поставить оценку')
    } finally {
      setBusyTask(null)
    }
  }

  async function rateBehavior(target: Participant, rating: number) {
    if (!requireCurrentParticipant()) {
      return
    }
    const rater = currentParticipant
    if (!rater) {
      return
    }
    if (rater.id === target.id) {
      setError('Оцениваем друг друга, не себя')
      return
    }
    setBusyBehavior(target.id)
    setError('')
    try {
      await api('/api/behavior-ratings', {
        method: 'POST',
        body: JSON.stringify({
          date: selectedDate,
          raterParticipantId: rater.id,
          targetParticipantId: target.id,
          rating,
          comment: '',
        }),
      })
      await loadData()
    } catch (behaviorError) {
      setError(behaviorError instanceof Error ? behaviorError.message : 'Не удалось сохранить оценку поведения')
    } finally {
      setBusyBehavior(null)
    }
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div className="topbar-actions">
          <section className="summary-grid topbar-summary" aria-label="Итоги текущего плана">
            <Metric label="На сегодня" value={totals.total} />
            <Metric label="Отмечено" value={totals.completed} />
            <Metric label="Подтверждено" value={totals.confirmed} />
            <Metric label="Начислено" value={`${Math.round(totals.reward)} ⭐`} />
          </section>
          <div className="date-card">
            <select aria-label="Раздел FamilyQuest" value={activeTab} onChange={(event) => setActiveTab(event.target.value as ActiveTab)}>
              {tabs.map((tab) => (
                <option key={tab.id} value={tab.id}>
                  {tab.label}
                </option>
              ))}
            </select>
            <input type="date" value={selectedDate} onChange={(event) => setSelectedDate(event.target.value)} />
          </div>
          <div className="user-switcher">
            <button className="current-user-card" onClick={() => setIsUserMenuOpen((isOpen) => !isOpen)}>
              <span className="big-avatar">{currentParticipant ? participantAvatar(currentParticipant) : '👀'}</span>
              <span>{currentParticipant ? currentParticipant.name : 'Просмотр'}</span>
            </button>
            {isUserMenuOpen && (
              <div className="user-menu">
                <button className={!currentParticipant ? 'active' : ''} onClick={enterViewMode}>
                  <span className="menu-avatar">👀</span>
                  <strong>Все</strong>
                  <small>Просмотр</small>
                </button>
                {participants.map((person) => (
                  <button
                    className={currentParticipant?.id === person.id ? 'active' : ''}
                    key={person.id}
                    onClick={() => askForParticipant(person)}
                  >
                    <span className="menu-avatar">{participantAvatar(person)}</span>
                    <strong>{person.name}</strong>
                    <small>{roleLabels[person.role]}</small>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </header>

      {error && <p className="notice">{error}</p>}

      {pinPrompt && (
        <div className="pin-backdrop" role="presentation">
          <form className="pin-dialog" onSubmit={verifyPin}>
            <div>
              <p className="eyebrow">PIN-код</p>
              <h2>{pinPrompt.participant.name}</h2>
            </div>
            <label>
              6 цифр
              <input
                autoFocus
                inputMode="numeric"
                maxLength={6}
                pattern="[0-9]{6}"
                type="password"
                value={pinPrompt.pin}
                onChange={(event) =>
                  setPinPrompt({
                    ...pinPrompt,
                    pin: event.target.value.replace(/\D/g, '').slice(0, 6),
                  })
                }
              />
            </label>
            <div className="pin-actions">
              <button type="button" onClick={() => setPinPrompt(null)}>
                Отмена
              </button>
              <button disabled={isCheckingPin || pinPrompt.pin.length !== 6} type="submit">
                Войти
              </button>
            </div>
          </form>
        </div>
      )}

      {activeTab === 'day' && (
        <>
          {!currentParticipant && (
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
          )}

          {(!currentParticipant || currentParticipant.name === 'Макс') && (
            <section className="max-progress">
              <div>
                <p className="eyebrow">Фокус для Макса</p>
                <h2>{maxCompleted}/{maxTasks.length} детских дел отмечено</h2>
              </div>
              <div className="progress-track" aria-label={`Прогресс Макса ${maxProgress}%`}>
                <span style={{ width: `${maxProgress}%` }} />
              </div>
            </section>
          )}

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
                      <div className="tag-row">
                        <span>{benefitLabels[task.benefitType]}</span>
                        <span>{executionLabels[task.executionMode]}</span>
                      </div>
                    </div>
                    <div className="task-actions">
                      {task.status === 'pending' && (
                        <button disabled={busyTask === task.id || currentParticipant?.id !== task.participantId} onClick={() => completeTask(task)}>
                          {currentParticipant?.id === task.participantId ? 'Сделано' : `Ждет ${task.personName}`}
                        </button>
                      )}
                      {task.status === 'completed' && (
                        <>
                          {currentParticipant && currentParticipant.id !== task.participantId && (
                            <div className="rating-buttons" aria-label="Оценка выполнения">
                              {[1, 2, 3, 4, 5].map((rating) => (
                                <button disabled={busyTask === task.id} key={rating} onClick={() => confirmTask(task, rating)}>
                                  {rating}
                                </button>
                              ))}
                            </div>
                          )}
                          {(!currentParticipant || currentParticipant.id === task.participantId) && (
                            <span className="action-hint">Ждет подтверждения</span>
                          )}
                        </>
                      )}
                      {task.status === 'confirmed' && <strong>{task.reward.toFixed(0)} ⭐</strong>}
                    </div>
                  </article>
                ))}
              </div>
            </div>

            <aside className="side-column">
              <Leaderboard title="Рейтинг дня" entries={dayLeaderboard} />

              {currentParticipant && (
                <section className="panel">
                  <h2>На подтверждение</h2>
                  <div className="review-list">
                    {tasksForReview.length === 0 && <p className="settings-note">Пока нет чужих дел, которые ждут оценки.</p>}
                    {tasksForReview.map((task) => (
                      <article className="review-card" key={task.id}>
                        <div>
                          <strong>{task.choreTitle}</strong>
                          <span>{task.personName}</span>
                        </div>
                        <div className="rating-buttons" aria-label={`Оценка выполнения ${task.choreTitle}`}>
                          {[1, 2, 3, 4, 5].map((rating) => (
                            <button disabled={busyTask === task.id} key={rating} onClick={() => confirmTask(task, rating)}>
                              {rating}
                            </button>
                          ))}
                        </div>
                      </article>
                    ))}
                  </div>
                </section>
              )}

              <section className="panel">
                <h2>Поведение дня</h2>
                <div className="behavior-list">
                  {!currentParticipant && <p className="settings-note">Выберите участника сверху, чтобы оценить атмосферу дня.</p>}
                  {currentParticipant &&
                    participants
                      .filter((person) => person.id !== currentParticipant.id)
                      .map((person) => (
                        <div className="behavior-row" key={person.id}>
                          <span>{person.name}</span>
                          <div className="rating-buttons" aria-label={`Оценка поведения ${person.name}`}>
                            {[1, 2, 3, 4, 5].map((rating) => (
                              <button disabled={busyBehavior === person.id} key={rating} onClick={() => rateBehavior(person, rating)}>
                                {rating}
                              </button>
                            ))}
                          </div>
                        </div>
                      ))}
                </div>
              </section>

              <section className="panel">
                <h2>Новое назначение</h2>
                <form className="stack-form" onSubmit={createAssignment}>
                  <AssignmentFields
                    chores={chores}
                    newAssignment={newAssignment}
                    participants={participants}
                    setNewAssignment={setNewAssignment}
                  />
                  <button type="submit">Назначить</button>
                </form>
              </section>
            </aside>
          </section>
        </>
      )}

      {activeTab === 'week' && (
        <section className="workspace">
          <div className="task-panel">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Неделя от {formatDate(weekStart(selectedDate))}</p>
                <h2>План недели</h2>
              </div>
            </div>
            <div className="week-grid">
              {weekPlan.map((day) => (
                <article className="week-day" key={day.day}>
                  <h3>{day.day}</h3>
                  {day.chores.length === 0 && <p>Нет назначений</p>}
                  {day.chores.slice(0, 5).map((assignment) => (
                    <span key={`${day.day}-${assignment.id}`}>
                      {assignment.personName}: {assignment.choreTitle}
                    </span>
                  ))}
                  {day.chores.length > 5 && <small>еще {day.chores.length - 5}</small>}
                </article>
              ))}
            </div>
          </div>
          <aside className="side-column">
            <Leaderboard title="Рейтинг недели" entries={weekLeaderboard} />
            <section className="panel">
              <h2>Назначить на неделю</h2>
              <form className="stack-form" onSubmit={createAssignment}>
                <AssignmentFields
                  chores={chores}
                  newAssignment={newAssignment}
                  participants={participants}
                  setNewAssignment={setNewAssignment}
                />
                <button type="submit">Назначить</button>
              </form>
            </section>
          </aside>
        </section>
      )}

      {activeTab === 'month' && (
        <section className="month-layout">
          <div className="section-heading">
            <div>
              <p className="eyebrow">{monthLabel(selectedDate)}</p>
              <h2>Общий месячный рейтинг</h2>
            </div>
          </div>
          <div className="month-grid">
            {monthLeaderboard.map((entry, index) => (
              <article className="month-card" key={entry.participantId}>
                <span className="rank">#{index + 1}</span>
                <h3>{entry.name}</h3>
                <strong>{entry.reward.toFixed(0)} ⭐</strong>
                <div className="month-progress">
                  <span style={{ width: `${completionPercent(entry)}%` }} />
                </div>
                <p>
                  {entry.tasksDone}/{entry.tasksAssigned} выполнено · {completionPercent(entry)}%
                </p>
                <small>Дела {entry.averageRating.toFixed(1)}/5 · Поведение {behaviorLabel(entry)}</small>
              </article>
            ))}
          </div>
          <Leaderboard title="Детали месяца" entries={monthLeaderboard} />
        </section>
      )}

      {activeTab === 'catalog' && (
        <section className="catalog">
          <div className="panel">
            <h2>Справочник обязанностей</h2>
            <div className="chore-grid">
              {chores.map((chore) => (
                <article key={chore.id}>
                  <h3>{chore.title}</h3>
                  <p>{chore.description}</p>
                  <div className="tag-row">
                    <span>{benefitLabels[chore.benefitType]}</span>
                    <span>{executionLabels[chore.executionMode]}</span>
                  </div>
                  <footer>
                    <span>{scheduleLabels[chore.schedule]}</span>
                    <strong>{chore.baseValue} ⭐</strong>
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
                  Когда
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
              <div className="form-row">
                <label>
                  Польза
                  <select
                    value={newChore.benefitType}
                    onChange={(event) => setNewChore({ ...newChore, benefitType: event.target.value as BenefitType })}
                  >
                    {Object.entries(benefitLabels).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Выполнение
                  <select
                    value={newChore.executionMode}
                    onChange={(event) => setNewChore({ ...newChore, executionMode: event.target.value as ExecutionMode })}
                  >
                    {Object.entries(executionLabels).map(([value, label]) => (
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
      )}

      {activeTab === 'users' && (
        <section className="users-grid">
          {participants.map((person) => (
            <article className="user-card" key={person.id}>
              <span className={`avatar ${person.role}`}>{person.name.slice(0, 1)}</span>
              <div>
                <h2>{person.name}</h2>
                <p>{roleLabels[person.role]}</p>
              </div>
              <dl>
                <div>
                  <dt>PIN</dt>
                  <dd>6 цифр, задан</dd>
                </div>
                <div>
                  <dt>Сегодня</dt>
                  <dd>{tasks.filter((task) => task.participantId === person.id).length} дел</dd>
                </div>
              </dl>
              <button disabled>Изменить PIN</button>
            </article>
          ))}
          <div className="panel">
            <h2>Настройки семьи</h2>
            <p className="settings-note">
              Сейчас участники создаются стартовыми данными. Следующим шагом сюда добавим смену PIN, новые профили и права родителей.
            </p>
          </div>
        </section>
      )}
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

function Leaderboard({ entries, title }: { entries: LeaderboardEntry[]; title: string }) {
  return (
    <section className="panel">
      <div className="section-heading compact">
        <h2>{title}</h2>
      </div>
      <ol className="leaderboard">
        {entries.map((entry) => (
          <li key={entry.participantId}>
            <span>{entry.name}</span>
            <strong>{entry.reward.toFixed(0)} ⭐</strong>
            <small>
              {entry.tasksDone}/{entry.tasksAssigned} дел · дела {entry.averageRating.toFixed(1)}/5 · поведение {behaviorLabel(entry)}
            </small>
          </li>
        ))}
      </ol>
    </section>
  )
}

function AssignmentFields({
  chores,
  newAssignment,
  participants,
  setNewAssignment,
}: {
  chores: Chore[]
  newAssignment: { choreId: number; participantId: number }
  participants: Participant[]
  setNewAssignment: (assignment: { choreId: number; participantId: number }) => void
}) {
  return (
    <>
      <label>
        Обязанность
        <select value={newAssignment.choreId} onChange={(event) => setNewAssignment({ ...newAssignment, choreId: Number(event.target.value) })}>
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
    </>
  )
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!response.ok) {
    const payload = await response.json().catch(() => null)
    throw new Error(payload?.error ?? `API error ${response.status}`)
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
    return `${task.reward.toFixed(0)} ⭐ · оценка ${task.averageRating.toFixed(1)}/5`
  }
  return `${taskBaseValue(assignments, task)} ⭐ за выполнение`
}

function formatDate(value: string) {
  const date = new Date(`${value}T00:00:00`)
  return new Intl.DateTimeFormat('ru-RU', {
    day: 'numeric',
    month: 'long',
    weekday: 'long',
  }).format(date)
}

function monthLabel(value: string) {
  const date = new Date(`${value}T00:00:00`)
  return new Intl.DateTimeFormat('ru-RU', {
    month: 'long',
    year: 'numeric',
  }).format(date)
}

function completionPercent(entry: LeaderboardEntry) {
  if (entry.tasksAssigned === 0) {
    return 0
  }
  return Math.round((entry.tasksDone / entry.tasksAssigned) * 100)
}

function behaviorLabel(entry: LeaderboardEntry) {
  if (entry.behaviorCount === 0) {
    return 'нет оценок'
  }
  return `${entry.behaviorRating.toFixed(1)}/5`
}

function participantAvatar(participant: Participant) {
  if (participant.name === 'Мама') {
    return '👩'
  }
  if (participant.name === 'Папа') {
    return '👨'
  }
  if (participant.name === 'Макс') {
    return '🧒'
  }
  return participant.role === 'child' ? '🙂' : '👤'
}

function weekStart(value: string) {
  const date = new Date(`${value}T00:00:00`)
  const weekday = date.getDay() || 7
  date.setDate(date.getDate() - (weekday - 1))
  return date.toISOString().slice(0, 10)
}

export default App
