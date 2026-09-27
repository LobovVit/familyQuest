import { useEffect, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import type { DeploymentInfo } from '../../domain/version'

export function AboutVersion() {
 const { gateway } = useRuntime()
 const [info, setInfo] = useState<DeploymentInfo | null>(null)
 const [error, setError] = useState(false)
 const [attempt, setAttempt] = useState(0)
 useEffect(() => {
  let active = true
  setError(false)
  void gateway.version().then(value => { if (active) setInfo(value) }).catch(() => { if (active) setError(true) })
  return () => { active = false }
 }, [gateway, attempt])
 const mismatch = info && [info.web, info.access].some(part => part && part.commit !== 'unknown' && info.server.commit !== 'unknown' && part.commit !== info.server.commit)
 const date = (value: string) => Number.isNaN(Date.parse(value)) ? 'не указано' : new Date(value).toLocaleString('ru-RU')
 return <footer className="app-version"><details>
  <summary>О приложении{info && !error ? ` · ${info.server.version}` : ''}</summary>
  {error ? <p>Не удалось проверить версию сервера. <button onClick={() => setAttempt(value => value + 1)}>Повторить</button></p> : info ? <>
   <p>Сервер: {info.server.version} · <code>{info.server.commit.slice(0, 12)}</code></p>
   <p>Сборка сервера: {date(info.server.builtAt)}. Запущен: {date(info.server.startedAt)}.</p>
   {info.access && <><p>Сервис доступа: {info.access.version} · <code>{info.access.commit.slice(0, 12)}</code></p><p>Сборка сервиса доступа: {date(info.access.builtAt)}. Запущен: {date(info.access.startedAt)}.</p></>}
   <p>Интерфейс: {info.web.version} · <code>{info.web.commit.slice(0, 12)}</code></p>
   <p>Сборка интерфейса: {date(info.web.builtAt)}.</p>
   {mismatch && <p role="status">Компоненты приложения разных сборок. Обновите страницу; если сообщение осталось, сообщите администратору.</p>}
  </> : <p>Проверяем версию…</p>}
 </details></footer>
}
