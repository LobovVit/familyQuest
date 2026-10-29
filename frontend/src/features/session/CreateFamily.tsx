import { useRef, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import type { Participant } from '../../domain/models'

export function CreateFamily({ onCreated }: { onCreated: (profiles: Participant[]) => void }) {
 const { gateway } = useRuntime()
 const submitting = useRef(false)
 const [busy, setBusy] = useState(false)
 const [error, setError] = useState('')
 return <form className="stack-form" onSubmit={async event => {
  event.preventDefault()
  if (submitting.current) return
  const form = event.currentTarget
  const values = new FormData(form)
  const name = String(values.get('familyName') ?? '').trim()
  const parentName = String(values.get('parentName') ?? '').trim()
  const pin = String(values.get('pin') ?? '')
  setError('')
  if (!name || !parentName || !/^\d{6}$/.test(pin)) { setError('Укажите название семьи, своё имя и PIN из 6 цифр.'); return }
  if (pin !== values.get('confirmPin')) { setError('PIN-коды не совпадают. Повторите ввод.'); return }
  submitting.current = true
  setBusy(true)
  try { onCreated(await gateway.registerFamily({ name, parentName, pin })) }
  catch (error) { setError(error instanceof Error ? error.message : 'Не удалось создать семью. Повторите попытку.') }
  finally { submitting.current = false; setBusy(false) }
 }}>
  <h2>Создайте свою семью</h2>
  <p>Выберите название для вашего семейного пространства и создайте первый профиль родителя.</p>
  <label>Название семьи<input name="familyName" placeholder="Например, Соколовы" autoComplete="off" required maxLength={80} disabled={busy} /></label>
  <label>Ваше имя<input name="parentName" autoComplete="given-name" required maxLength={80} disabled={busy} /></label>
  <label>PIN родителя<input name="pin" type="password" inputMode="numeric" autoComplete="new-password" pattern="[0-9]{6}" minLength={6} maxLength={6} required disabled={busy} aria-describedby="family-pin-help" /></label>
  <label>Повторите PIN<input name="confirmPin" type="password" inputMode="numeric" autoComplete="new-password" pattern="[0-9]{6}" minLength={6} maxLength={6} required disabled={busy} /></label>
  <p id="family-pin-help">6 цифр для входа в ваш семейный профиль. Не сообщайте PIN детям.</p>
  <p>Создание семьи не активирует подписку. Для добавления детей и новых дел потребуется действующий доступ к сервису.</p>
  {error && <p role="alert">{error}</p>}
  <button disabled={busy} type="submit">{busy ? 'Создаём семью…' : 'Создать семью'}</button>
  <button type="button" disabled={busy} onClick={() => void gateway.logout()}>Выйти из аккаунта</button>
 </form>
}
