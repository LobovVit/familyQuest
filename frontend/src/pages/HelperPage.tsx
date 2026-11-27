import { useEffect, useState, type FormEvent } from 'react'
import { useRuntime } from '../application/runtime'
import type { HelperTask } from '../domain/helper'
import '../features/helper/helper.css'

const taskDate = new Intl.DateTimeFormat('ru-RU', {
 dateStyle: 'long',
 timeZone: 'UTC',
})

export function HelperPage() {
 const {gateway} = useRuntime()
 const [input, setInput] = useState('')
 const [token, setToken] = useState('')
 const [tasks, setTasks] = useState<HelperTask[] | null>(null)
 const [message, setMessage] = useState('')

 useEffect(() => {
  if (!token) return
  let active = true
  let request: AbortController | undefined
  async function refresh() {
   request?.abort()
   const current = new AbortController()
   request = current
   try {
    const items = await gateway.helperTasks(token, current.signal)
    if (active && !current.signal.aborted) {
     setTasks(items)
     setMessage('')
    }
   } catch {
    if (active && !current.signal.aborted) {
     setTasks(null)
     setMessage('Доступ недоступен. Проверьте код или свяжитесь с родителем.')
    }
   }
  }
  void refresh()
  const timer = window.setInterval(refresh, 15_000)
  window.addEventListener('focus', refresh)
  return () => {
   active = false
   request?.abort()
   window.clearInterval(timer)
   window.removeEventListener('focus', refresh)
  }
 }, [gateway, token])

 function enter(event: FormEvent) {
  event.preventDefault()
  setToken(input.trim())
  setInput('')
  setMessage('')
 }

 function exit() {
  setToken('')
  setTasks(null)
  setMessage('')
 }

 return <main className="helper-page">
  <h1>Дела помощника</h1>
  {token ? <>
   {tasks && (tasks.length > 0 ? <ul className="helper-task-list">{tasks.map((task, index) => <li key={index}>
    <strong>{task.title}</strong>
    <time dateTime={task.date}>{taskDate.format(new Date(task.date + 'T00:00:00Z'))}</time>
   </li>)}</ul> : <p>Назначенных дел пока нет.</p>)}
   {!tasks && <p role="status">{message || 'Проверяем доступ…'}</p>}
   <button className="helper-exit" onClick={exit}>Выйти</button>
  </> : <form onSubmit={enter}>
   <p>Введите код, который вам передал родитель.</p>
   <label>Код доступа <input type="password" autoComplete="off" value={input} maxLength={100} onChange={event => setInput(event.target.value)} required /></label>
   <button disabled={!input.trim()}>Открыть дела</button>
  </form>}
 </main>
}
