import { type Dispatch, type SetStateAction, useCallback, useEffect, useMemo, useState } from 'react'
import './App.css'

const API_URL = import.meta.env.VITE_API_URL ?? ''

type Participant = {
  id: number
  name: string
  role: 'parent' | 'child' | 'school'
  active: boolean
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
  participantIds: number[]
  participantNames: string[]
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
  choreDescription: string
  personName: string
  dueDate: string
  schedule: Schedule
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
  behaviorSmiles: number
}

type PinPrompt = {
  participant: Participant
  pin: string
}

type Schedule = 'once' | 'daily' | 'weekly' | 'monthly'
type TimeWindow = '' | 'morning' | 'day' | 'evening'
type BenefitType = 'self' | 'family' | 'care' | 'home'
type ExecutionMode = 'assigned' | 'together' | 'adult_child' | 'anyone'
type ActiveTab = 'day' | 'catalog' | 'users'
type ChoreDraft = Omit<Chore, 'id' | 'participantNames'>
type RewardPeriod = 'day' | 'week' | 'month'
type RewardType = 'champion' | 'stars' | 'smiles'

type Reward = {
  id: number
  title: string
  description: string
  period: RewardPeriod
  rewardType: RewardType
  starCost: number
  smileCost: number
  participantIds: number[]
  participantNames: string[]
}

const scheduleLabels: Record<Schedule, string> = {
  once: 'Разово',
  daily: 'Ежедневно',
  weekly: 'Еженедельно',
  monthly: 'Ежемесячно',
}

const windowLabels: Record<TimeWindow, string> = {
  '': 'Когда угодно',
  morning: 'Утро',
  day: 'День',
  evening: 'Вечер',
}

const roleLabels: Record<Participant['role'], string> = {
  parent: 'Взрослый',
  child: 'Дошкольник',
  school: 'Школьник',
}

const rewardPeriodLabels: Record<RewardPeriod, string> = {
  day: 'День',
  week: 'Неделя',
  month: 'Месяц',
}

const rewardTypeLabels: Record<RewardType, string> = {
  champion: 'Чемпионская',
  stars: 'За звездочки',
  smiles: 'За улыбки',
}

const benefitLabels: Record<BenefitType, string> = {
  self: 'Для себя',
  family: 'Для семьи',
  care: 'Забота',
  home: 'Дом',
}

const tabs: Array<{ id: ActiveTab; label: string; adultsOnly?: boolean }> = [
  { id: 'day', label: 'Планер' },
  { id: 'catalog', label: 'Справочник обязанностей', adultsOnly: true },
  { id: 'users', label: 'Настройки пользователей', adultsOnly: true },
]

function App() {
  const [selectedDate, setSelectedDate] = useState(() => new Date().toISOString().slice(0, 10))
  const [participants, setParticipants] = useState<Participant[]>([])
  const [chores, setChores] = useState<Chore[]>([])
  const [assignments, setAssignments] = useState<Assignment[]>([])
  const [rewards, setRewards] = useState<Reward[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [dayLeaderboard, setDayLeaderboard] = useState<LeaderboardEntry[]>([])
  const [weekLeaderboard, setWeekLeaderboard] = useState<LeaderboardEntry[]>([])
  const [monthLeaderboard, setMonthLeaderboard] = useState<LeaderboardEntry[]>([])
  const [activeTab, setActiveTab] = useState<ActiveTab>('day')
  const [busyTask, setBusyTask] = useState<number | null>(null)
  const [busyBehavior, setBusyBehavior] = useState<number | null>(null)
  const [expandedTaskDescriptions, setExpandedTaskDescriptions] = useState<Record<number, boolean>>({})
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')
  const [currentParticipant, setCurrentParticipant] = useState<Participant | null>(null)
  const [pinPrompt, setPinPrompt] = useState<PinPrompt | null>(null)
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false)
  const [isCheckingPin, setIsCheckingPin] = useState(false)
  const [editingChoreId, setEditingChoreId] = useState<number | 'new' | null>(null)
  const [choreDraft, setChoreDraft] = useState<ChoreDraft>(() => emptyChoreDraft())
  const [newParticipant, setNewParticipant] = useState({ name: '', role: 'child' as Participant['role'], pin: '' })
  const [pinEdit, setPinEdit] = useState<{ participantId: number; pin: string } | null>(null)
  const [newReward, setNewReward] = useState({
    title: '',
    description: '',
    period: 'week' as RewardPeriod,
    rewardType: 'champion' as RewardType,
    starCost: 100,
    smileCost: 20,
    participantIds: [] as number[],
  })

  const loadData = useCallback(async () => {
    setError('')
    try {
      const [participantsData, choresData, assignmentsData, rewardsData, tasksData, dayLeaderboardData, weekLeaderboardData, monthLeaderboardData] =
        await Promise.all([
          api<Participant[]>('/api/participants'),
          api<Chore[]>('/api/chores'),
          api<Assignment[]>('/api/assignments'),
          api<Reward[]>('/api/rewards'),
          api<Task[]>(`/api/tasks?date=${selectedDate}`),
          api<LeaderboardEntry[]>(`/api/leaderboard?period=day&date=${selectedDate}`),
          api<LeaderboardEntry[]>(`/api/leaderboard?period=week&date=${selectedDate}`),
          api<LeaderboardEntry[]>(`/api/leaderboard?period=month&date=${selectedDate}`),
        ])

      setParticipants(participantsData)
      setChores(choresData)
      setAssignments(assignmentsData)
      setRewards(rewardsData)
      setTasks(tasksData)
      setDayLeaderboard(dayLeaderboardData)
      setWeekLeaderboard(weekLeaderboardData)
      setMonthLeaderboard(monthLeaderboardData)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Не удалось загрузить данные')
    } finally {
      setIsLoading(false)
    }
  }, [selectedDate])

  useEffect(() => {
    void loadData()
  }, [loadData])

  const availableTabs = useMemo(() => {
    return tabs.filter((tab) => !tab.adultsOnly || currentParticipant?.role === 'parent')
  }, [currentParticipant])

  useEffect(() => {
    if (!availableTabs.some((tab) => tab.id === activeTab)) {
      setActiveTab('day')
    }
  }, [activeTab, availableTabs])

  const filteredTasks = useMemo(() => {
    if (currentParticipant) {
      return tasks.filter((task) => task.participantId === currentParticipant.id)
    }
    return tasks
  }, [currentParticipant, tasks])

  const maxTasks = useMemo(() => tasks.filter((task) => task.personName === 'Макс'), [tasks])
  const maxCompleted = maxTasks.filter((task) => task.status !== 'pending').length
  const maxProgress = maxTasks.length === 0 ? 0 : Math.round((maxCompleted / maxTasks.length) * 100)

  const tasksForReview = useMemo(() => {
    if (!currentParticipant) {
      return []
    }
    return tasks.filter((task) => task.status === 'completed' && task.participantId !== currentParticipant.id)
  }, [currentParticipant, tasks])

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
    setError('')
    setIsUserMenuOpen(false)
  }

  function shiftSelectedDate(days: number) {
    setSelectedDate((currentDate) => {
      const date = new Date(`${currentDate}T00:00:00`)
      date.setDate(date.getDate() + days)
      const month = String(date.getMonth() + 1).padStart(2, '0')
      const day = String(date.getDate()).padStart(2, '0')
      return `${date.getFullYear()}-${month}-${day}`
    })
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

  function startNewChore() {
    if (!requireCurrentParticipant()) {
      return
    }
    setError('')
    setChoreDraft(emptyChoreDraft())
    setEditingChoreId('new')
    setActiveTab('catalog')
  }

  function startEditChore(chore: Chore) {
    if (!requireCurrentParticipant()) {
      return
    }
    setError('')
    setChoreDraft({
      title: chore.title,
      description: chore.description,
      schedule: chore.schedule,
      timeWindow: chore.timeWindow,
      benefitType: chore.benefitType,
      executionMode: chore.executionMode,
      baseValue: chore.baseValue,
      participantIds: chore.participantIds ?? [],
    })
    setEditingChoreId(chore.id)
  }

  function cancelEditChore() {
    setEditingChoreId(null)
    setChoreDraft(emptyChoreDraft())
  }

  function toggleDraftParticipant(participantId: number) {
    setChoreDraft((current) => {
      const hasParticipant = current.participantIds.includes(participantId)
      return {
        ...current,
        participantIds: hasParticipant
          ? current.participantIds.filter((id) => id !== participantId)
          : [...current.participantIds, participantId],
      }
    })
  }

  async function saveChore() {
    if (!requireCurrentParticipant()) {
      return
    }
    if (!choreDraft.title.trim()) {
      setError('Добавьте название обязанности')
      return
    }
    if (choreDraft.participantIds.length === 0) {
      setError('Выберите хотя бы одного участника')
      return
    }
    setError('')
    try {
      const payload = {
        ...choreDraft,
        title: choreDraft.title.trim(),
        timeWindow: choreDraft.schedule === 'daily' ? choreDraft.timeWindow : '',
        executionMode: 'assigned' as ExecutionMode,
        baseValue: Math.max(1, Number(choreDraft.baseValue) || 1),
      }
      await api(editingChoreId === 'new' ? '/api/chores' : `/api/chores/${editingChoreId}`, {
        method: editingChoreId === 'new' ? 'POST' : 'PUT',
        body: JSON.stringify(payload),
      })
      cancelEditChore()
      await loadData()
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Не удалось сохранить обязанность')
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

  async function createParticipant(event: React.FormEvent) {
    event.preventDefault()
    if (!requireCurrentParticipant()) {
      return
    }
    if (!newParticipant.name.trim() || newParticipant.pin.length !== 6) {
      setError('Укажите имя и PIN из 6 цифр')
      return
    }
    setError('')
    try {
      await api('/api/participants', {
        method: 'POST',
        body: JSON.stringify({ ...newParticipant, name: newParticipant.name.trim() }),
      })
      setNewParticipant({ name: '', role: 'child', pin: '' })
      await loadData()
    } catch (participantError) {
      setError(participantError instanceof Error ? participantError.message : 'Не удалось добавить пользователя')
    }
  }

  async function deleteParticipant(participant: Participant) {
    if (!requireCurrentParticipant()) {
      return
    }
    setError('')
    try {
      await api(`/api/participants/${participant.id}`, { method: 'DELETE' })
      if (currentParticipant?.id === participant.id) {
        enterViewMode()
      }
      await loadData()
    } catch (participantError) {
      setError(participantError instanceof Error ? participantError.message : 'Не удалось удалить пользователя')
    }
  }

  async function saveParticipantPIN(participant: Participant) {
    if (!requireCurrentParticipant()) {
      return
    }
    if (!pinEdit || pinEdit.participantId !== participant.id || pinEdit.pin.length !== 6) {
      setError('PIN должен содержать 6 цифр')
      return
    }
    setError('')
    try {
      await api(`/api/participants/${participant.id}/pin`, {
        method: 'PUT',
        body: JSON.stringify({ pin: pinEdit.pin }),
      })
      setPinEdit(null)
      await loadData()
    } catch (pinError) {
      setError(pinError instanceof Error ? pinError.message : 'Не удалось изменить PIN')
    }
  }

  function toggleRewardParticipant(participantId: number) {
    setNewReward((current) => {
      const hasParticipant = current.participantIds.includes(participantId)
      return {
        ...current,
        participantIds: hasParticipant
          ? current.participantIds.filter((id) => id !== participantId)
          : [...current.participantIds, participantId],
      }
    })
  }

  async function createReward(event: React.FormEvent) {
    event.preventDefault()
    if (!requireCurrentParticipant()) {
      return
    }
    if (!newReward.title.trim()) {
      setError('Добавьте название награды')
      return
    }
    if (newReward.participantIds.length === 0) {
      setError('Выберите хотя бы одного пользователя для награды')
      return
    }
    setError('')
    try {
      await api('/api/rewards', {
        method: 'POST',
        body: JSON.stringify({
          ...newReward,
          title: newReward.title.trim(),
          starCost: newReward.rewardType === 'champion' ? 0 : Math.max(1, Number(newReward.starCost) || 1),
          smileCost: newReward.rewardType === 'smiles' ? Math.max(1, Number(newReward.smileCost) || 1) : 0,
          ...(newReward.rewardType !== 'stars' ? { starCost: 0 } : {}),
        }),
      })
      setNewReward({ title: '', description: '', period: 'week', rewardType: 'champion', starCost: 100, smileCost: 20, participantIds: [] })
      await loadData()
    } catch (rewardError) {
      setError(rewardError instanceof Error ? rewardError.message : 'Не удалось добавить награду')
    }
  }

  async function deleteReward(reward: Reward) {
    if (!requireCurrentParticipant()) {
      return
    }
    setError('')
    try {
      await api(`/api/rewards/${reward.id}`, { method: 'DELETE' })
      await loadData()
    } catch (rewardError) {
      setError(rewardError instanceof Error ? rewardError.message : 'Не удалось удалить награду')
    }
  }

  function renderPlanControls() {
    return (
      <>
        <div className="date-card">
          <select aria-label="Раздел FamilyQuest" value={activeTab} onChange={(event) => setActiveTab(event.target.value as ActiveTab)}>
            {availableTabs.map((tab) => (
              <option key={tab.id} value={tab.id}>
                {tab.label}
              </option>
            ))}
          </select>
          <div className="date-stepper" aria-label="Выбор даты плана">
            <button aria-label="Предыдущий день" className="date-arrow" type="button" onClick={() => shiftSelectedDate(-1)}>
              ‹
            </button>
            <strong>{formatDate(selectedDate)}</strong>
            <button aria-label="Следующий день" className="date-arrow" type="button" onClick={() => shiftSelectedDate(1)}>
              ›
            </button>
          </div>
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
      </>
    )
  }

  return (
    <main className="app-shell">
      {(!currentParticipant || activeTab !== 'day') && (
        <header className="topbar">
          <div className={`topbar-actions ${currentParticipant ? 'compact' : ''}`}>
            {!currentParticipant ? (
              <section className="topbar-focus" aria-label="Фокус для Макса">
                <div>
                  <p className="eyebrow">Фокус для Макса</p>
                  <h2>{maxCompleted}/{maxTasks.length} дел отмечено</h2>
                </div>
                <div className="progress-track" aria-label={`Прогресс Макса ${maxProgress}%`}>
                  <span style={{ width: `${maxProgress}%` }} />
                </div>
              </section>
            ) : null}
            {renderPlanControls()}
          </div>
        </header>
      )}

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
          {!currentParticipant ? (
            <section className="view-dashboard">
              <Leaderboard title="Рейтинг дня" entries={dayLeaderboard} />
              <Leaderboard title="Рейтинг недели" entries={weekLeaderboard} />
              <Leaderboard title="Рейтинг месяца" entries={monthLeaderboard} />
            </section>
          ) : (
            <section className="workspace">
              <div className="side-controls">{renderPlanControls()}</div>
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
                        {task.choreDescription.trim() && (
                          <div className="task-description">
                            <p>{expandedTaskDescriptions[task.id] ? task.choreDescription : previewText(task.choreDescription, 30)}</p>
                            {task.choreDescription.trim().length > 30 && (
                              <button
                                type="button"
                                onClick={() =>
                                  setExpandedTaskDescriptions((expanded) => ({
                                    ...expanded,
                                    [task.id]: !expanded[task.id],
                                  }))
                                }
                              >
                                {expandedTaskDescriptions[task.id] ? 'Скрыть' : 'Показать все'}
                              </button>
                            )}
                          </div>
                        )}
                        <p>{task.schedule === 'daily' ? windowLabels[task.timeWindow] : scheduleLabels[task.schedule]} · {taskRewardLabel(assignments, task)}</p>
                        <div className="tag-row">
                          <span>{benefitLabels[task.benefitType]}</span>
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
                            {currentParticipant.id !== task.participantId && (
                              <div className="rating-buttons" aria-label="Оценка выполнения">
                                {[1, 2, 3, 4, 5].map((rating) => (
                                  <button disabled={busyTask === task.id} key={rating} onClick={() => confirmTask(task, rating)}>
                                    {rating}
                                  </button>
                                ))}
                              </div>
                            )}
                            {currentParticipant.id === task.participantId && <span className="action-hint">Ждет подтверждения</span>}
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

                <section className="panel">
                  <h2>Поведение дня</h2>
                  <div className="behavior-list">
                    {participants
                      .filter((person) => person.id !== currentParticipant.id)
                      .map((person) => (
                        <div className="behavior-row" key={person.id}>
                          <span>{person.name}</span>
                          <div className="rating-buttons" aria-label={`Оценка поведения ${person.name}`}>
                            {[1, 2, 3, 4, 5].map((rating) => (
                              <button disabled={busyBehavior === person.id} key={rating} onClick={() => rateBehavior(person, rating)}>
                                {rating} 🙂
                              </button>
                            ))}
                          </div>
                        </div>
                      ))}
                  </div>
                </section>
              </aside>
            </section>
          )}
        </>
      )}

      {activeTab === 'catalog' && (
        <section className="catalog single">
          <div className="panel">
            <div className="section-heading compact">
              <h2>Справочник обязанностей</h2>
              <button type="button" onClick={startNewChore}>
                Добавить
              </button>
            </div>
            <div className="chore-grid">
              {editingChoreId === 'new' && (
                <article className="chore-card editing">
                  <ChoreEditor
                    draft={choreDraft}
                    onCancel={cancelEditChore}
                    onSave={saveChore}
                    onToggleParticipant={toggleDraftParticipant}
                    participants={participants}
                    setDraft={setChoreDraft}
                  />
                </article>
              )}
              {chores.map((chore) => (
                <article className={`chore-card ${editingChoreId === chore.id ? 'editing' : ''}`} key={chore.id}>
                  {editingChoreId === chore.id ? (
                    <ChoreEditor
                      draft={choreDraft}
                      onCancel={cancelEditChore}
                      onSave={saveChore}
                      onToggleParticipant={toggleDraftParticipant}
                      participants={participants}
                      setDraft={setChoreDraft}
                    />
                  ) : (
                    <>
                      <div className="chore-card-head">
                        <h3>{chore.title}</h3>
                        <button aria-label={`Редактировать ${chore.title}`} className="icon-button" onClick={() => startEditChore(chore)} type="button">
                          ✏
                        </button>
                      </div>
                      <p>{chore.description}</p>
                      <div className="tag-row">
                        <span>{benefitLabels[chore.benefitType]}</span>
                      </div>
                      <div className="assignee-row">
                        {chore.participantNames?.length ? (
                          chore.participantNames.map((name) => <span key={name}>{avatarForName(name)} {name}</span>)
                        ) : (
                          <span>Не привязано</span>
                        )}
                      </div>
                      <footer>
                        <span>{scheduleLabels[chore.schedule]} · {windowLabels[chore.timeWindow]}</span>
                        <strong>{chore.baseValue} ⭐</strong>
                      </footer>
                    </>
                  )}
                </article>
              ))}
            </div>
          </div>
        </section>
      )}

      {activeTab === 'users' && (
        <section className="users-grid">
          {participants.map((person) => (
            <article className="user-card" key={person.id}>
              <span className={`avatar ${person.role}`}>{participantAvatar(person)}</span>
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
              {pinEdit?.participantId === person.id ? (
                <div className="pin-edit">
                  <input
                    autoFocus
                    inputMode="numeric"
                    maxLength={6}
                    value={pinEdit.pin}
                    onChange={(event) =>
                      setPinEdit({
                        participantId: person.id,
                        pin: event.target.value.replace(/\D/g, '').slice(0, 6),
                      })
                    }
                  />
                  <button disabled={pinEdit.pin.length !== 6} type="button" onClick={() => saveParticipantPIN(person)}>
                    Сохранить
                  </button>
                  <button type="button" onClick={() => setPinEdit(null)}>
                    Отмена
                  </button>
                </div>
              ) : (
                <div className="user-actions">
                  <button type="button" onClick={() => setPinEdit({ participantId: person.id, pin: '' })}>
                    Изменить PIN
                  </button>
                  <button type="button" onClick={() => deleteParticipant(person)}>
                    Удалить
                  </button>
                </div>
              )}
            </article>
          ))}
          <section className="panel">
            <h2>Добавить пользователя</h2>
            <form className="stack-form" onSubmit={createParticipant}>
              <label>
                Имя
                <input value={newParticipant.name} onChange={(event) => setNewParticipant({ ...newParticipant, name: event.target.value })} />
              </label>
              <div className="form-row">
                <label>
                  Роль
                  <select
                    value={newParticipant.role}
                    onChange={(event) => setNewParticipant({ ...newParticipant, role: event.target.value as Participant['role'] })}
                  >
                    <option value="child">Дошкольник</option>
                    <option value="school">Школьник</option>
                    <option value="parent">Взрослый</option>
                  </select>
                </label>
                <label>
                  PIN
                  <input
                    inputMode="numeric"
                    maxLength={6}
                    value={newParticipant.pin}
                    onChange={(event) => setNewParticipant({ ...newParticipant, pin: event.target.value.replace(/\D/g, '').slice(0, 6) })}
                  />
                </label>
              </div>
              <button type="submit">Добавить пользователя</button>
            </form>
          </section>

          <section className="panel rewards-panel">
            <div className="section-heading compact">
              <h2>Награды</h2>
            </div>
            <div className="reward-list">
              {rewards.length === 0 && <p className="settings-note">Добавьте первые награды для обмена звездочек или чемпионства.</p>}
              {rewards.map((reward) => (
                <article className="reward-card" key={reward.id}>
                  <div>
                    <h3>{reward.title}</h3>
                    <p>{reward.description}</p>
                  </div>
                  <div className="tag-row">
                    <span>{rewardTypeLabels[reward.rewardType]}</span>
                    <span>{rewardPeriodLabels[reward.period]}</span>
                    {reward.rewardType === 'stars' && <span>{reward.starCost} ⭐</span>}
                    {reward.rewardType === 'smiles' && <span>{reward.smileCost} 🙂</span>}
                  </div>
                  <div className="assignee-row">
                    {reward.participantNames.map((name) => (
                      <span key={name}>{avatarForName(name)} {name}</span>
                    ))}
                  </div>
                  <button type="button" onClick={() => deleteReward(reward)}>
                    Удалить
                  </button>
                </article>
              ))}
            </div>
          </section>

          <section className="panel rewards-panel">
            <h2>Добавить награду</h2>
            <form className="stack-form" onSubmit={createReward}>
              <label>
                Название
                <input value={newReward.title} onChange={(event) => setNewReward({ ...newReward, title: event.target.value })} />
              </label>
              <label>
                Описание
                <textarea value={newReward.description} onChange={(event) => setNewReward({ ...newReward, description: event.target.value })} />
              </label>
              <div className="form-row">
                <label>
                  Период
                  <select value={newReward.period} onChange={(event) => setNewReward({ ...newReward, period: event.target.value as RewardPeriod })}>
                    {Object.entries(rewardPeriodLabels).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Тип
                  <select
                    value={newReward.rewardType}
                    onChange={(event) => setNewReward({ ...newReward, rewardType: event.target.value as RewardType })}
                  >
                    {Object.entries(rewardTypeLabels).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              {newReward.rewardType === 'stars' && (
                <label>
                  Стоимость в звездочках
                  <input
                    min="1"
                    type="number"
                    value={newReward.starCost}
                    onChange={(event) => setNewReward({ ...newReward, starCost: Number(event.target.value) })}
                  />
                </label>
              )}
              {newReward.rewardType === 'smiles' && (
                <label>
                  Стоимость в улыбках
                  <input
                    min="1"
                    type="number"
                    value={newReward.smileCost}
                    onChange={(event) => setNewReward({ ...newReward, smileCost: Number(event.target.value) })}
                  />
                </label>
              )}
              <div className="participant-picker" aria-label="Пользователи награды">
                {participants.map((person) => (
                  <label className={newReward.participantIds.includes(person.id) ? 'active' : ''} key={person.id}>
                    <input checked={newReward.participantIds.includes(person.id)} onChange={() => toggleRewardParticipant(person.id)} type="checkbox" />
                    <span>{participantAvatar(person)}</span>
                    <strong>{person.name}</strong>
                  </label>
                ))}
              </div>
              <button type="submit">Добавить награду</button>
            </form>
          </section>
        </section>
      )}
    </main>
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
            <span className="leaderboard-avatar">{avatarForName(entry.name)}</span>
            <span className="leaderboard-name">{entry.name}</span>
            <strong>{entry.reward.toFixed(0)} ⭐ · {entry.behaviorSmiles} 🙂</strong>
            <small>
              {entry.tasksDone}/{entry.tasksAssigned} дел · дела {entry.averageRating.toFixed(1)}/5 · поведение {behaviorLabel(entry)}
            </small>
          </li>
        ))}
      </ol>
    </section>
  )
}

function ChoreEditor({
  draft,
  onCancel,
  onSave,
  onToggleParticipant,
  participants,
  setDraft,
}: {
  draft: ChoreDraft
  onCancel: () => void
  onSave: () => void
  onToggleParticipant: (participantId: number) => void
  participants: Participant[]
  setDraft: Dispatch<SetStateAction<ChoreDraft>>
}) {
  return (
    <div className="chore-editor">
      <label>
        Название
        <input value={draft.title} onChange={(event) => setDraft({ ...draft, title: event.target.value })} />
      </label>
      <label>
        Описание
        <textarea value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
      </label>
      <div className="form-row">
        <label>
          Периодичность
          <select
            value={draft.schedule}
            onChange={(event) => {
              const schedule = event.target.value as Schedule
              setDraft({ ...draft, schedule, timeWindow: schedule === 'daily' ? draft.timeWindow : '' })
            }}
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
            disabled={draft.schedule !== 'daily'}
            value={draft.schedule === 'daily' ? draft.timeWindow : ''}
            onChange={(event) => setDraft({ ...draft, timeWindow: event.target.value as TimeWindow })}
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
        Польза
        <select value={draft.benefitType} onChange={(event) => setDraft({ ...draft, benefitType: event.target.value as BenefitType })}>
          {Object.entries(benefitLabels).map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Базовая ценность
        <input min="1" type="number" value={draft.baseValue} onChange={(event) => setDraft({ ...draft, baseValue: Number(event.target.value) })} />
      </label>
      <div className="participant-picker" aria-label="Участники обязанности">
        {participants.map((person) => (
          <label className={draft.participantIds.includes(person.id) ? 'active' : ''} key={person.id}>
            <input checked={draft.participantIds.includes(person.id)} onChange={() => onToggleParticipant(person.id)} type="checkbox" />
            <span>{participantAvatar(person)}</span>
            <strong>{person.name}</strong>
          </label>
        ))}
      </div>
      <div className="editor-actions">
        <button type="button" onClick={onCancel}>
          Отмена
        </button>
        <button type="button" onClick={onSave}>
          Сохранить
        </button>
      </div>
    </div>
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

function previewText(value: string, limit: number) {
  const normalized = value.trim().replace(/\s+/g, ' ')
  if (normalized.length <= limit) {
    return normalized
  }
  return `${normalized.slice(0, limit).trim()}...`
}

function formatDate(value: string) {
  const date = new Date(`${value}T00:00:00`)
  const months = ['Января', 'Февраля', 'Марта', 'Апреля', 'Мая', 'Июня', 'Июля', 'Августа', 'Сентября', 'Октября', 'Ноября', 'Декабря']
  return `${date.getDate()} ${months[date.getMonth()]} ${date.getFullYear()}`
}

function behaviorLabel(entry: LeaderboardEntry) {
  if (entry.behaviorCount === 0) {
    return 'нет оценок'
  }
  return `${entry.behaviorRating.toFixed(1)}/5`
}

function emptyChoreDraft(): ChoreDraft {
  return {
    title: '',
    description: '',
    schedule: 'daily',
    timeWindow: '',
    benefitType: 'self',
    executionMode: 'assigned',
    baseValue: 50,
    participantIds: [],
  }
}

function participantAvatar(participant: Participant) {
  return avatarForName(participant.name, participant.role)
}

function avatarForName(name: string, role?: Participant['role']) {
  if (name === 'Мама') {
    return '👩'
  }
  if (name === 'Папа') {
    return '👨'
  }
  if (name === 'Макс') {
    return '🧒'
  }
  if (role === 'school') {
    return '🎒'
  }
  return role === 'child' ? '🙂' : '👤'
}

export default App
