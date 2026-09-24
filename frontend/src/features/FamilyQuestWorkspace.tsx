import { MyDay } from './today/MyDay'
import { WorkspaceNavigation } from './navigation/WorkspaceNavigation'
import { workspaceSections, type WorkspaceSection } from './navigation/sections'
import { useParentConfirmation } from '../application/useParentConfirmation'
import { ParentConfirmation } from './session/ParentConfirmation'
import { Devices } from './session/Devices'
import type { LoginOptions } from '../domain/devices'
import { ReadingTraining } from './learning/ReadingTraining'
import { MathTraining } from './learning/MathTraining'
import { ActivityRewards } from './learning/ActivityRewards'
import { FamilyOverview } from './overview/FamilyOverview'
import { Sports } from './sports/Sports'
import { FamilyLife } from './family/FamilyLife'
import { type ChangeEvent, useEffect, useMemo, useState } from 'react'
import '../App.css'
import './navigation/workspace.css'
import type { Chore, ChoreDraft, ExecutionMode, Participant, Reward, RewardPeriod, RewardType, Task } from '../domain/models'
import { taskProgress } from '../domain/policies'
import { useSession } from '../application/useSession'
import { useFamilyQuestData } from '../application/useFamilyQuestData'
import { useFamilyQuestActions } from '../application/useFamilyQuestActions'
import { useRuntime } from '../application/runtime'
import { localDate } from '../domain/date'
import { canReviewTask } from '../domain/policies'
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

type ActiveTab = WorkspaceSection
const tabs = workspaceSections

export function FamilyQuestWorkspace() {
 const { downloadBackup, restoreBackup } = useRuntime()
  const [selectedDate, setSelectedDate] = useState(() => localDate(new Date()))
  const [activeTab, setActiveTab] = useState<ActiveTab>('today')
  const [busyTask, setBusyTask] = useState<number | null>(null)
  const [busyBehavior, setBusyBehavior] = useState<number | null>(null)
  const [isBackupBusy, setIsBackupBusy] = useState(false)
  const [error, setError] = useState('')
  const { participant: currentParticipant, login: establishSession, logout } = useSession()
  const confirmation = useParentConfirmation()
  const [loginOptions,setLoginOptions] = useState<LoginOptions>({remember:false,deviceName:''})
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

  const data = useFamilyQuestData(selectedDate, currentParticipant)
  const { participants, chores, assignments, rewards, tasks, dayLeaderboard, weekLeaderboard, monthLeaderboard, behaviorRatings, isLoading } = data
  const actions = useFamilyQuestActions(data.refresh)
  useEffect(() => { if (data.loadError) setError(data.loadError) }, [data.loadError])

  const availableTabs = useMemo(() => {
    return tabs.filter((tab) => (tab.id !== 'today' || currentParticipant?.role === 'child' || currentParticipant?.role === 'parent') && (!['math', 'reading'].includes(tab.id) || currentParticipant?.role === 'child') && (!['family', 'sport', 'earned'].includes(tab.id) || currentParticipant?.role === 'parent' || currentParticipant?.role === 'child') && (!tab.adultsOnly || currentParticipant?.role === 'parent'))
  }, [currentParticipant])

  useEffect(() => {
    if (!availableTabs.some((tab) => tab.id === activeTab)) {
      setActiveTab(currentParticipant && currentParticipant.role !== 'school' ? 'today' : 'day')
    }
  }, [activeTab, availableTabs, currentParticipant])

  const filteredTasks = useMemo(() => {
    if (currentParticipant) {
      return tasks.filter((task) => task.participantId === currentParticipant.id)
    }
    return tasks
  }, [currentParticipant, tasks])

  const completedTasks = currentParticipant ? tasks.filter((task) => task.status !== 'pending').length : dayLeaderboard.reduce((sum, entry) => sum + entry.tasksDone, 0)
  const totalTasks = currentParticipant ? tasks.length : dayLeaderboard.reduce((sum, entry) => sum + entry.tasksAssigned, 0)
  const overallProgress = currentParticipant ? taskProgress(tasks) : totalTasks > 0 ? Math.min(100, Math.round(completedTasks / totalTasks * 100)) : 0

  const tasksForReview = useMemo(() => {
    if (!currentParticipant) {
      return []
    }
    return tasks.filter((task) => canReviewTask(currentParticipant, task))
  }, [currentParticipant, tasks])

  function askForParticipant(participant: Participant) {
    if (currentParticipant?.id === participant.id) {
      setIsUserMenuOpen(false)
      return
    }
    setError('')
    setIsUserMenuOpen(false)
    setLoginOptions({remember:false,deviceName:`Устройство · ${participant.name}`,parentId:participants.find(p=>p.role==='parent')?.id,parentPin:''})
    setPinPrompt({ participant, pin: '' })
  }

  async function enterViewMode() {
    try { await logout() } catch { setError('Не удалось выйти. Проверьте соединение и повторите.'); return }
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
      const session = await actions.login(pinPrompt.participant.id, pinPrompt.pin, loginOptions)
      establishSession(session)
      setActiveTab(session.participant.role === 'school' ? 'day' : 'today')
      setPinPrompt(null)
      setLoginOptions({remember:false,deviceName:''})
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
      await actions.completeTask(task)
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
      await actions.confirmTask(task, rating)
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
      await actions.rateBehavior(selectedDate, target.id, rating)
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
      const proof = await confirmation.ask()
      if (!proof) return
      await actions.changePin(participant.id, pinEdit.pin, proof)
      if (participant.id === currentParticipant?.id) await enterViewMode()
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
    const confirmed = window.confirm('Загрузка файла полностью заменит текущих пользователей, обязанности, задачи, рейтинги, семейные карточки, воспоминания и историю выполнения. Продолжить?')
    if (!confirmed) {
      return
    }
    setIsBackupBusy(true)
    setError('')
    try {
      const proof = await confirmation.ask()
      if (!proof) return
      await restoreBackup(file, proof)
      await enterViewMode()
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
          <div className="date-stepper" aria-label="Выбор даты плана">
            <button aria-label="Предыдущий день" className="date-arrow" type="button" onClick={() => shiftSelectedDate(-1)}>
              ‹
            </button>
            <strong>{formatDate(selectedDate)}</strong>
            <button aria-label="Следующий день" className="date-arrow" type="button" onClick={() => shiftSelectedDate(1)}>
              ›
            </button>
          </div>
          <button className="today-shortcut" disabled={selectedDate === localDate(new Date())} onClick={() => setSelectedDate(localDate(new Date()))}>Сегодня</button>
        </div>
        <UserMenu current={currentParticipant} participants={participants} open={isUserMenuOpen} onToggle={() => setIsUserMenuOpen(value => !value)} onView={enterViewMode} onSelect={askForParticipant} />
      </>
    )
  }

  return (
    <main className="app-shell">
      <WorkspaceNavigation items={availableTabs} active={activeTab} onSelect={setActiveTab} />
      <div className="workspace-content">
      <header className="workspace-toolbar"><div className="workspace-title"><p className="eyebrow">{currentParticipant ? 'Семейное пространство' : 'Общий прогресс семьи'}</p><h2>{availableTabs.find(tab => tab.id === activeTab)?.label ?? 'Планер'}</h2>{!currentParticipant && <small>{completedTasks}/{totalTasks} дел отмечено · {overallProgress}%</small>}</div>{renderPlanControls()}</header>

      {error && <p className="notice">{error}</p>}

      {<ParentConfirmation confirmation={confirmation} />}
      {pinPrompt && <PinDialog participants={participants} options={loginOptions} onOptions={setLoginOptions} error={error} participant={pinPrompt.participant} pin={pinPrompt.pin} busy={isCheckingPin} onPin={pin => setPinPrompt({...pinPrompt,pin})} onCancel={() => setPinPrompt(null)} onSubmit={verifyPin} />}

      {activeTab === 'day' && <Planner participant={currentParticipant} participants={participants} tasks={tasks} filteredTasks={filteredTasks} reviewTasks={tasksForReview} assignments={assignments} ratings={behaviorRatings} day={dayLeaderboard} week={weekLeaderboard} month={monthLeaderboard} date={selectedDate} loading={isLoading} busyTask={busyTask} busyBehavior={busyBehavior} controls={null} onComplete={completeTask} onConfirm={confirmTask} onRate={rateBehavior} />}

      {!currentParticipant && activeTab === 'day' && <FamilyOverview date={selectedDate} participants={participants} />}

      {activeTab === 'today' && currentParticipant && ['parent', 'child'].includes(currentParticipant.role) && <MyDay participant={currentParticipant} tasks={tasks} assignments={assignments} summary={dayLeaderboard.find(entry => entry.participantId === currentParticipant.id)} loading={isLoading} busyTask={busyTask} reviewCount={tasksForReview.length} onComplete={completeTask} onNavigate={setActiveTab} />}
      {activeTab === 'reading' && currentParticipant?.role === 'child' && <ReadingTraining key={currentParticipant.id} />}
      {activeTab === 'math' && currentParticipant?.role === 'child' && <MathTraining key={currentParticipant.id} onReward={() => { void data.refresh() }} />}
      {activeTab === 'earned' && currentParticipant && ['parent', 'child'].includes(currentParticipant.role) && <ActivityRewards key={currentParticipant.id} />}

      {activeTab === 'sport' && currentParticipant && (currentParticipant.role === 'parent' || currentParticipant.role === 'child') && <Sports key={currentParticipant.id} current={currentParticipant} participants={participants} date={selectedDate} onProgress={() => { void data.refresh() }} />}

      {activeTab === 'family' && currentParticipant && (currentParticipant.role === 'parent' || currentParticipant.role === 'child') && <FamilyLife key={currentParticipant.id} current={currentParticipant} participants={participants} date={selectedDate} onProgress={() => { void data.refresh() }} />}

      {currentParticipant?.role === 'parent' && activeTab === 'catalog' && <Catalog chores={chores} editingId={editingChoreId} onAdd={startNewChore} onEdit={startEditChore} newEditor={<ChoreEditor draft={choreDraft} onCancel={cancelEditChore} onSave={saveChore} onToggleParticipant={toggleDraftParticipant} participants={participants} setDraft={setChoreDraft} />} editor={() => <ChoreEditor draft={choreDraft} onCancel={cancelEditChore} onSave={saveChore} onToggleParticipant={toggleDraftParticipant} participants={participants} setDraft={setChoreDraft} />} />}

      {currentParticipant?.role === 'parent' && activeTab === 'users' && <Devices confirm={confirmation.ask} onCurrentRevoked={() => { void enterViewMode() }} />}
      {currentParticipant?.role === 'parent' && activeTab === 'users' && <Settings participants={participants} tasks={tasks} rewards={rewards} pinEdit={pinEdit} setPinEdit={setPinEdit} newParticipant={newParticipant} setNewParticipant={setNewParticipant} newReward={newReward} setNewReward={setNewReward} backupBusy={isBackupBusy} onSavePin={saveParticipantPIN} onDeleteParticipant={deleteParticipant} onCreateParticipant={createParticipant} onExport={exportBackup} onImport={importBackup} onDeleteReward={deleteReward} onCreateReward={createReward} onToggleRewardParticipant={toggleRewardParticipant} />}
      </div>
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
