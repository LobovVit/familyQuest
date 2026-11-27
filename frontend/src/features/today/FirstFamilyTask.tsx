import { useEffect, useRef, useState, type FormEvent } from 'react'
import { localDate } from '../../domain/date'
import { useRuntime } from '../../application/runtime'
import type { Assignment, ChoreDraft, Participant } from '../../domain/models'
import type { FamilySubscription } from '../../domain/subscription'

type Props = {
 childrenProfiles: Participant[]
 assignments: Assignment[]
 ready: boolean
 visible: boolean
 onAddChild: (name: string, pin: string) => Promise<Participant>
 onAssign: (draft: ChoreDraft) => Promise<unknown>
 onOpenProfile: (profile: Participant) => void
 onSettings: () => void
}

export function FirstFamilyTask({ childrenProfiles, assignments, ready, visible, onAddChild, onAssign, onOpenProfile, onSettings }: Props) {
 const { gateway } = useRuntime()
 const [subscription, setSubscription] = useState<FamilySubscription>()
 const [accessError, setAccessError] = useState(false)
 const [attempt, setAttempt] = useState(0)
 const [name, setName] = useState('')
 const [pin, setPin] = useState('')
 const [title, setTitle] = useState('')
 const [selectedId, setSelectedId] = useState(0)
 const [assignedProfile, setAssignedProfile] = useState<Participant>()
 const [busy, setBusy] = useState(false)
 const [error, setError] = useState('')
 const submitting = useRef(false)
 const heading = useRef<HTMLHeadingElement>(null)
 const hasChildAssignment = assignments.some(a => childrenProfiles.some(child => child.id === a.participantId))
 const needsSetup = ready && !hasChildAssignment && !assignedProfile
 const checkAccess = needsSetup && visible
 const child = childrenProfiles.find(profile => profile.id === selectedId) ?? childrenProfiles[0]
 const hasChild = !!child

 useEffect(() => {
  if (!checkAccess) return
  let active = true
  void gateway.subscription().then(value => {
   if (active) { setSubscription(value); setAccessError(false) }
  }).catch(() => {
   if (active) { setSubscription(undefined); setAccessError(true) }
  })
  return () => { active = false }
 }, [gateway, checkAccess, attempt])

 const focusAfterSave = useRef(false)
 useEffect(() => {
  if (visible && !busy && focusAfterSave.current && (hasChild || assignedProfile)) {
   heading.current?.focus()
   focusAfterSave.current = false
  }
 }, [visible, busy, hasChild, assignedProfile])

 async function submit(event: FormEvent<HTMLFormElement>) {
  event.preventDefault()
  if (submitting.current || !needsSetup || !subscription?.access) return
  setError('')
  if (child ? !title.trim() : !name.trim() || !/^\d{6}$/.test(pin)) {
   setError(child ? 'Введите название дела.' : 'Укажите имя и PIN из 6 цифр.')
   return
  }
  submitting.current = true
  setBusy(true)
  try {
   if (child) {
    await onAssign({ title: title.trim(), description: '', schedule: 'once', onceDate: localDate(new Date()), timeWindow: '', benefitType: 'self', executionMode: 'assigned', baseValue: 20, participantIds: [child.id] })
    setAssignedProfile(child)
    setTitle('')
   } else {
    const profile = await onAddChild(name.trim(), pin)
    setSelectedId(profile.id)
    setName('')
    setPin('')
   }
   focusAfterSave.current = true
  } catch (error) {
   setError(error instanceof Error ? error.message : 'Не удалось сохранить. Повторите попытку.')
  } finally {
   submitting.current = false
   setBusy(false)
  }
 }

 if (!visible || (!needsSetup && !busy && !assignedProfile)) return null
 return <section className="panel first-family-task" aria-labelledby="first-family-task-heading">
  <h2 ref={heading} tabIndex={-1} id="first-family-task-heading">
   {assignedProfile ? 'Дело назначено' : child ? 'Назначьте дело на сегодня' : 'Добавьте ребёнка'}
  </h2>
  {assignedProfile ? <div className="stack-form">
    <p role="status">Дело появилось в разделе «Мой день» профиля «{assignedProfile.name}».</p>
    {childrenProfiles.some(profile => profile.id === assignedProfile.id) && <button type="button" onClick={() => onOpenProfile(assignedProfile)}>Открыть профиль ребёнка</button>}
   </div> :
   accessError ? <>
    <p role="alert">Не удалось проверить доступ семьи.</p>
    <button type="button" onClick={() => { setAccessError(false); setAttempt(value => value + 1) }}>Повторить</button>
   </> : !subscription ? <p role="status">Проверяем доступ…</p> :
   !subscription.access || (!child && subscription.activeChildren >= subscription.childLimit) ? <>
    <p>{subscription.access ? 'В семье достигнут лимит детских профилей.' : 'Для добавления детей и дел нужен действующий доступ.'}</p>
    <button type="button" onClick={onSettings}>Открыть настройки семьи</button>
   </> : <form className="stack-form" onSubmit={submit}>
    {child ? <>
     {childrenProfiles.length > 1
      ? <label>Кому<select value={child.id} disabled={busy} onChange={event => setSelectedId(Number(event.target.value))}>
       {childrenProfiles.map(profile => <option key={profile.id} value={profile.id}>{profile.name}</option>)}
      </select></label>
      : <p>Для профиля «{child.name}»</p>}
     <small>Разовое дело · 20 ⭐ за подтверждение с оценкой 5</small>
     <label>Название дела<input required maxLength={160} value={title} disabled={busy} placeholder="Например, полить цветы" onChange={event => setTitle(event.target.value)} /></label>
    </> : <>
     <p>Ребёнок сможет открыть свой профиль по PIN и отметить выполненное дело.</p>
     <label>Имя ребёнка<input required maxLength={80} value={name} disabled={busy} onChange={event => setName(event.target.value)} /></label>
     <label>PIN ребёнка<input required type="password" inputMode="numeric" autoComplete="new-password" pattern="[0-9]{6}" maxLength={6} value={pin} disabled={busy} onChange={event => setPin(event.target.value.replace(/\D/g, ''))} /></label>
     <small>Придумайте PIN из 6 цифр и сообщите его ребёнку.</small>
    </>}
    {error && <p className="notice" role="alert">{error}</p>}
    <button type="submit" disabled={busy}>{busy ? 'Сохраняем…' : child ? 'Назначить дело' : 'Добавить ребёнка'}</button>
   </form>}
 </section>
}
