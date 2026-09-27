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
   <div className="account-password">
    <label htmlFor="account-password">Пароль</label>
    <div className="account-password-field">
     <input id="account-password" name="password" type={visible ? 'text' : 'password'} autoComplete="current-password" required maxLength={72} disabled={busy} />
     <button className="account-password-toggle" type="button" disabled={busy} aria-label={visible ? 'Скрыть пароль' : 'Показать пароль'} title={visible ? 'Скрыть пароль' : 'Показать пароль'} aria-pressed={visible} aria-controls="account-password" onClick={() => setVisible(value => !value)}>
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
       <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
       <circle cx="12" cy="12" r="3" />
       {visible && <path d="m3 3 18 18" />}
      </svg>
     </button>
    </div>
   </div>
   <p>Используйте пароль взрослого аккаунта. PIN семейного профиля понадобится после входа.</p>
   {error && <p role="alert">{error}</p>}
   <button disabled={busy} type="submit">{busy ? 'Входим…' : 'Войти'}</button>
  </form>
  <p>Для подключения семьи и восстановления доступа обратитесь к оператору сервиса. Детский профиль выбирается после входа взрослого.</p>
 </section></main>
}
