import { useEffect, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import { useSession } from '../../application/useSession'
import '../family/family.css'
import type { Participant } from '../../domain/models'

export function AccountLogin({ sso = false }: { sso?: boolean }) {
 const { gateway } = useRuntime()
 const { login } = useSession()
 const [busy, setBusy] = useState(false)
 const [error, setError] = useState('')
 const [visible, setVisible] = useState(false)
 const [remember,setRemember]=useState(false),[profiles,setProfiles]=useState<Participant[]|null>(null),[selected,setSelected]=useState(''),[pin,setPin]=useState('')
 useEffect(() => {
  if (!sso) return
  const complete=new URLSearchParams(window.location.search).get('sso') === 'complete'
  let active = true
  setBusy(true)
  void (complete ? gateway.ssoSession() : gateway.restoreAccount()).then(result => {
   if (!active) return
   const url = new URL(window.location.href); url.searchParams.delete('sso')
   window.history.replaceState(null, '', url.pathname + url.search + url.hash)
   setProfiles(result)
  }).catch(e => { if (active) setError(e instanceof Error ? e.message : 'Не удалось завершить единый вход') })
    .finally(() => { if (active) setBusy(false) })
  return () => { active = false }
 }, [sso, gateway, login])
 if (sso) return <main className="account-entry"><section className="panel">
  <h1>FamilyQuest 🌳</h1>
  <p>Один аккаунт для всех подключённых сервисов. Подписка на каждый сервис оформляется отдельно.</p>
  {error && <p role="alert">{error}</p>}
  {!profiles && !busy && <label className="remember-choice"><input type="checkbox" checked={remember} onChange={e=>setRemember(e.target.checked)}/>Запомнить это устройство на 30 дней</label>}
  {profiles && <form className="stack-form" onSubmit={async e=>{e.preventDefault();setBusy(true);setError('');try{login(await gateway.selectAccountProfile(Number(selected),pin))}catch(e){setError(e instanceof Error?e.message:'Не удалось войти')}finally{setPin('');setBusy(false)}}}>
   <label>Семейный профиль<select value={selected} onChange={e=>{setSelected(e.target.value);setPin('')}} required><option value="">Выберите профиль</option>{profiles.filter(p=>p.active).map(p=><option value={p.id} key={p.id}>{p.name}</option>)}</select></label>
   <label>PIN профиля<input type="password" inputMode="numeric" autoComplete="off" value={pin} maxLength={6} onChange={e=>setPin(e.target.value.replace(/\D/g,'').slice(0,6))}/></label>
   <button disabled={busy||!selected||pin.length!==6}>Открыть профиль</button>
   <button type="button" disabled={busy} onClick={()=>void gateway.logout()}>Выйти из всех сервисов</button>
  </form>}
  {busy ? <p role="status">Завершаем вход…</p> : !profiles && <a className="button" href={remember ? "/api/account/authorize?remember=1" : "/api/account/authorize"}>Войти с единым аккаунтом</a>}
  <p>После входа выберите семейный профиль. Для переключения профиля используется его PIN.</p>
 </section></main>
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
