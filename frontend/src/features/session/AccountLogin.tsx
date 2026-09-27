import { useState } from 'react'
import { useRuntime } from '../../application/runtime'
import { useSession } from '../../application/useSession'
import '../family/family.css'

export function AccountLogin() {
 const { gateway } = useRuntime()
 const { login } = useSession()
 const [busy, setBusy] = useState(false)
 const [error, setError] = useState('')
 const [visible, setVisible] = useState(false)
 return <main className="account-entry"><section className="panel">
  <h1>FamilyQuest 🌳</h1><p>Войдите в пространство вашей семьи</p>
  <form className="stack-form" onSubmit={async e => {
   e.preventDefault()
   if (busy) return
   const form = e.currentTarget
   // Read actual fields: autofill may not emit React change events.
   // Читаем сами поля: автозаполнение может не вызвать события React.
   const values = new FormData(form)
   const email = String(values.get('email') ?? '').trim()
   const password = String(values.get('password') ?? '')
   setBusy(true); setError('')
   try { login(await gateway.accountLogin(email, password)) }
   catch (e) { setError(e instanceof Error ? e.message : 'Не удалось войти') }
   finally {
    setBusy(false)
    const field = form.elements.namedItem('password') as HTMLInputElement | null
    if (field) field.value = ''
   }
  }}>
   <label>Почта взрослого<input name="email" type="email" autoComplete="username" required maxLength={254} disabled={busy} /></label>
   <label>Пароль<input name="password" type={visible ? 'text' : 'password'} autoComplete="current-password" required maxLength={72} disabled={busy} /></label>
   <button type="button" aria-pressed={visible} onClick={() => setVisible(value => !value)}>{visible ? 'Скрыть пароль' : 'Показать пароль'}</button>
   <p>Используйте пароль взрослого аккаунта. PIN семейного профиля понадобится после входа.</p>
   {error && <p role="alert">{error}</p>}
   <button disabled={busy} type="submit">{busy ? 'Входим…' : 'Войти'}</button>
  </form>
  <p>Для подключения семьи и восстановления доступа обратитесь к оператору сервиса. Детский профиль выбирается после входа взрослого.</p>
 </section></main>
}
