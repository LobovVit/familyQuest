import { useState } from 'react'
import { useRuntime } from '../../application/runtime'
import { useSession } from '../../application/useSession'
import '../family/family.css'
export function AccountLogin() {
 const { gateway } = useRuntime(), { login } = useSession()
 const [email, setEmail] = useState(''), [password, setPassword] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState('')
 return <main className="account-entry"><section className="panel"><h1>FamilyQuest 🌳</h1><p>Войдите в пространство вашей семьи</p><form className="stack-form" onSubmit={async e => {e.preventDefault(); if (busy) return; setBusy(true); setError(''); try {login(await gateway.accountLogin(email, password))} catch(e) {setError(e instanceof Error ? e.message : 'Не удалось войти')} finally {setBusy(false); setPassword('')}}}>
 <label>Почта взрослого<input type="email" autoComplete="username" value={email} onChange={e => setEmail(e.target.value)} required maxLength={254} /></label>
 <label>Пароль<input type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} required maxLength={72} /></label>
 {error && <p role="alert">{error}</p>}<button disabled={busy} type="submit">{busy ? 'Входим…' : 'Войти'}</button></form><p>Для подключения семьи и восстановления доступа обратитесь к оператору сервиса. Детский профиль выбирается после входа взрослого.</p></section></main>
}
