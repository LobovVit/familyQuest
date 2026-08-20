import { type ChangeEvent, useEffect, useMemo, useState } from 'react'
import '../App.css'
import type { Chore, ChoreDraft, ExecutionMode, Participant, Reward, RewardPeriod, RewardType, Task } from '../domain/models'
import { taskProgress } from '../domain/policies'
import { useSession } from '../application/useSession'
import { useFamilyQuestData } from '../application/useFamilyQuestData'
import { useFamilyQuestActions } from '../application/useFamilyQuestActions'
import { downloadBackup, restoreBackup } from '../infrastructure/backup'
import { Planner } from '../features/planner/Planner'
import { Catalog } from '../features/catalog/Catalog'
import { ChoreEditor } from './catalog/ChoreEditor'
import { UserMenu } from './session/UserMenu'
import { PinDialog } from './session/PinDialog'
import { Settings } from './settings/Settings'

type PinPrompt = {
  participant: Participant
  pin: string
}

type ActiveTab = 'day' | 'catalog' | 'users'

const tabs: Array<{ id: ActiveTab; label: string; adultsOnly?: boolean }> = [
  { id: 'day', label: 'Планер' },
  { id: 'catalog', label: 'Справочник обязанностей', adultsOnly: true },
  { id: 'users', label: 'Настройки пользователей', adultsOnly: true },
]

export function FamilyQuestWorkspace() {
  const [selectedDate, setSelectedDate] = useState(() => new Date().toISOString().slice(0, 10))
  const [activeTab, setActiveTab] = useState<ActiveTab>('day')
  const [busyTask, setBusyTask] = useState<number | null>(null)
  const [busyBehavior, setBusyBehavior] = useState<number | null>(null)
  const [isBackupBusy, setIsBackupBusy] = useState(false)
  const [error, setError] = useState('')
  const { participant: currentParticipant, login: establishSession, logout } = useSession()
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

  const data = useFamilyQuestData(selectedDate, currentParticipant !== null)
  const { participants, chores, assignments, rewards, tasks, dayLeaderboard, weekLeaderboard, monthLeaderboard, behaviorRatings, setBehaviorRatings, isLoading } = data
  const actions = useFamilyQuestActions(data.refresh)
  useEffect(() => { if (data.loadError) setError(data.loadError) }, [data.loadError])

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

  const completedTasks = tasks.filter((task) => task.status !== 'pending').length
  const overallProgress = taskProgress(tasks)

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
    logout()
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
      const session = await actions.login(pinPrompt.participant.id, pinPrompt.pin)
      establishSession(session)
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
    if (editingChoreId === null) return
    setError('')
    try {
      const payload = {
        ...choreDraft,
        title: choreDraft.title.trim(),
        timeWindow: choreDraft.schedule === 'daily' ? choreDraft.timeWindow : '',
        executionMode: 'assigned' as ExecutionMode,
        baseValue: Math.max(1, Number(choreDraft.baseValue) || 1),
      }
      await actions.saveChore(editingChoreId, payload)
      cancelEditChore()
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
      await actions.completeTask(task, currentParticipant.id)
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
      await actions.confirmTask(task, reviewer.id, rating)
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
      const savedRating = await actions.rateBehavior(selectedDate, rater.id, target.id, rating)
      setBehaviorRatings((ratings) => [
        ...ratings.filter((item) => item.raterParticipantId !== rater.id || item.targetParticipantId !== target.id),
        savedRating,
      ])
      await data.refresh()
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
      await actions.createParticipant({ ...newParticipant, name: newParticipant.name.trim() })
      await data.loadParticipants()
      setNewParticipant({ name: '', role: 'child', pin: '' })
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
      await actions.deleteParticipant(participant.id)
      await data.loadParticipants()
      if (currentParticipant?.id === participant.id) {
        enterViewMode()
      }
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
      await actions.changePin(participant.id, pinEdit.pin)
      setPinEdit(null)
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
      await actions.createReward({
          ...newReward,
          title: newReward.title.trim(),
          starCost: newReward.rewardType === 'champion' ? 0 : Math.max(1, Number(newReward.starCost) || 1),
          smileCost: newReward.rewardType === 'smiles' ? Math.max(1, Number(newReward.smileCost) || 1) : 0,
          ...(newReward.rewardType !== 'stars' ? { starCost: 0 } : {}),
      })
      setNewReward({ title: '', description: '', period: 'week', rewardType: 'champion', starCost: 100, smileCost: 20, participantIds: [] })
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
      await actions.deleteReward(reward.id)
    } catch (rewardError) {
      setError(rewardError instanceof Error ? rewardError.message : 'Не удалось удалить награду')
    }
  }

  async function exportBackup() {
    if (!requireCurrentParticipant()) {
      return
    }
    setIsBackupBusy(true)
    setError('')
    try {
      await downloadBackup(selectedDate)
    } catch (backupError) {
      setError(backupError instanceof Error ? backupError.message : 'Не удалось выгрузить данные')
    } finally {
      setIsBackupBusy(false)
    }
  }

  async function importBackup(event: ChangeEvent<HTMLInputElement>) {
    if (!requireCurrentParticipant()) {
      event.target.value = ''
      return
    }
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) {
      return
    }
    const confirmed = window.confirm('Загрузка файла полностью заменит текущих пользователей, обязанности, задачи, рейтинги и историю выполнения. Продолжить?')
    if (!confirmed) {
      return
    }
    setIsBackupBusy(true)
    setError('')
    try {
      await restoreBackup(file)
      enterViewMode()
      await data.loadParticipants()
      await data.refresh()
    } catch (backupError) {
      setError(backupError instanceof Error ? backupError.message : 'Не удалось загрузить данные из файла')
    } finally {
      setIsBackupBusy(false)
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
        <UserMenu current={currentParticipant} participants={participants} open={isUserMenuOpen} onToggle={() => setIsUserMenuOpen(value => !value)} onView={enterViewMode} onSelect={askForParticipant} />
      </>
    )
  }

  return (
    <main className="app-shell">
      {(!currentParticipant || activeTab !== 'day') && (
        <header className="topbar">
          <div className={`topbar-actions ${currentParticipant ? 'compact' : ''}`}>
            {!currentParticipant ? (
              <section className="topbar-focus" aria-label="Общий прогресс семьи">
                <div>
                  <p className="eyebrow">Прогресс семьи</p>
                  <h2>{completedTasks}/{tasks.length} дел отмечено</h2>
                </div>
                <div className="progress-track" aria-label={`Общий прогресс ${overallProgress}%`}>
                  <span style={{ width: `${overallProgress}%` }} />
                </div>
              </section>
            ) : null}
            {renderPlanControls()}
          </div>
        </header>
      )}

      {error && <p className="notice">{error}</p>}

      {pinPrompt && <PinDialog participant={pinPrompt.participant} pin={pinPrompt.pin} busy={isCheckingPin} onPin={pin => setPinPrompt({...pinPrompt,pin})} onCancel={() => setPinPrompt(null)} onSubmit={verifyPin} />}

      {activeTab === 'day' && <Planner participant={currentParticipant} participants={participants} tasks={tasks} filteredTasks={filteredTasks} reviewTasks={tasksForReview} assignments={assignments} ratings={behaviorRatings} day={dayLeaderboard} week={weekLeaderboard} month={monthLeaderboard} date={selectedDate} loading={isLoading} busyTask={busyTask} busyBehavior={busyBehavior} controls={renderPlanControls()} onComplete={completeTask} onConfirm={confirmTask} onRate={rateBehavior} />}

      {activeTab === 'catalog' && <Catalog chores={chores} editingId={editingChoreId} onAdd={startNewChore} onEdit={startEditChore} newEditor={<ChoreEditor draft={choreDraft} onCancel={cancelEditChore} onSave={saveChore} onToggleParticipant={toggleDraftParticipant} participants={participants} setDraft={setChoreDraft} />} editor={() => <ChoreEditor draft={choreDraft} onCancel={cancelEditChore} onSave={saveChore} onToggleParticipant={toggleDraftParticipant} participants={participants} setDraft={setChoreDraft} />} />}

      {activeTab === 'users' && <Settings participants={participants} tasks={tasks} rewards={rewards} pinEdit={pinEdit} setPinEdit={setPinEdit} newParticipant={newParticipant} setNewParticipant={setNewParticipant} newReward={newReward} setNewReward={setNewReward} backupBusy={isBackupBusy} onSavePin={saveParticipantPIN} onDeleteParticipant={deleteParticipant} onCreateParticipant={createParticipant} onExport={exportBackup} onImport={importBackup} onDeleteReward={deleteReward} onCreateReward={createReward} onToggleRewardParticipant={toggleRewardParticipant} />}
    </main>
  )
}

function formatDate(value: string) {
  const date = new Date(`${value}T00:00:00`)
  const months = ['Января', 'Февраля', 'Марта', 'Апреля', 'Мая', 'Июня', 'Июля', 'Августа', 'Сентября', 'Октября', 'Ноября', 'Декабря']
  return `${date.getDate()} ${months[date.getMonth()]} ${date.getFullYear()}`
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
