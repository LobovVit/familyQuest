import { useEffect, useState, type FormEvent } from 'react'
import { useRuntime } from '../../application/runtime'
import type { HelperAccess } from '../../domain/helper'
import type { Task } from '../../domain/models'
import './helper.css'

export function HelperAccessPanel({date, tasks, loading}: {date: string; tasks: Task[]; loading: boolean}) {
 const {gateway} = useRuntime()
 const [access, setAccess] = useState<HelperAccess[]>([])
 const [label, setLabel] = useState('')
 const [selection, setSelection] = useState<{date: string; ids: number[]}>({date, ids: []})
 const [token, setToken] = useState<{id: number; value: string} | null>(null)
 const [error, setError] = useState('')
 const [busy, setBusy] = useState(true)
 const taskIds = selection.date === date ? selection.ids.filter(id => tasks.some(task => task.id === id)) : []

 useEffect(() => {
  let active = true
  void gateway.helperAccess().then(items => {
   if (active) setAccess(items)
  }).catch(() => {
   if (active) setError('Не удалось загрузить доступы')
  }).finally(() => {
   if (active) setBusy(false)
  })
  return () => { active = false }
 }, [gateway])

 async function create(event: FormEvent) {
  event.preventDefault()
  setBusy(true)
  setError('')
  try {
   const result = await gateway.createHelperAccess({label, taskIds})
   setAccess(previous => [result.access, ...previous])
   setToken({id: result.access.id, value: result.token})
   setLabel('')
   setSelection({date, ids: []})
  } catch {
   setError('Не удалось выдать доступ. Проверьте выбранные дела и повторите.')
  } finally {
   setBusy(false)
  }
 }

 async function revoke(id: number) {
  setBusy(true)
  setError('')
  try {
   await gateway.revokeHelperAccess(id)
   setAccess(previous => previous.filter(item => item.id !== id))
   setToken(previous => previous?.id === id ? null : previous)
  } catch {
   setError('Не удалось отозвать доступ. Повторите попытку.')
  } finally {
   setBusy(false)
  }
 }

 return <section className="helper-panel" aria-labelledby="helper-heading">
  <h2 id="helper-heading">Семейный помощник</h2>
  <form onSubmit={create}>
   <label>Метка доступа <input value={label} maxLength={60} onChange={event => setLabel(event.target.value)} required /></label>
   <fieldset disabled={busy || loading}>
    <legend>Дела на {date}</legend>
    {tasks.map(task => <label key={task.id}>
     <input type="checkbox" checked={taskIds.includes(task.id)} disabled={taskIds.length >= 30 && !taskIds.includes(task.id)}
      onChange={event => setSelection({date, ids: event.target.checked ? [...taskIds, task.id] : taskIds.filter(id => id !== task.id)})} />
     {task.choreTitle} · {task.personName} · {task.dueDate}
    </label>)}
    {!loading && tasks.length === 0 && <p>На эту дату дел нет.</p>}
   </fieldset>
   <p>Помощник увидит названия и даты выбранных дел. Используйте названия без личных данных.</p>
   <button disabled={busy || loading || taskIds.length === 0}>Выдать доступ на 24 часа</button>
  </form>
  {token && <div>
   <label>Код доступа <input readOnly value={token.value} onFocus={event => event.target.select()} /></label>
   <p>Код показан один раз. Для входа помощник вводит его на странице <a href="/helper" target="_blank" rel="noreferrer">«Дела помощника»</a>.</p>
   <button onClick={() => setToken(null)}>Скрыть код</button>
  </div>}
  {access.length > 0 && <ul>{access.map(item => <li key={item.id}>
   <span>{item.label} · до {new Date(item.expiresAt).toLocaleString('ru')}</span>
   <button disabled={busy} onClick={() => void revoke(item.id)}>Отозвать доступ</button>
  </li>)}</ul>}
  {error && <p role="alert">{error}</p>}
 </section>
}
