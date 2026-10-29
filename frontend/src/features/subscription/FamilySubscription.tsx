import { useEffect, useRef, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import type { FamilySubscription as Subscription } from '../../domain/subscription'
export function FamilySubscription() {
 const { gateway } = useRuntime()
 const [value, setValue] = useState<Subscription | null>(null)
 const [name, setName] = useState('')
 const [error, setError] = useState('')
 const [saved, setSaved] = useState(false)
 const [busy, setBusy] = useState(false)
 const submitting = useRef(false)
 useEffect(() => {
  let active = true
  void gateway.subscription().then(value => { if (active) { setValue(value); setName(value.name) } }).catch(() => { if (active) setError('Не удалось загрузить подписку') })
  return () => { active = false }
 }, [gateway])
 return <section className="panel"><h2>Семья и подписка</h2>
  {error && <p role="alert">{error}</p>}
  {value && <>
   <form className="stack-form" onSubmit={async event => {
    event.preventDefault()
    if (submitting.current || !name.trim()) return
    submitting.current = true; setBusy(true); setError(''); setSaved(false)
    try {
     const result = await gateway.renameFamily(name.trim())
     setValue({ ...value, name: result.name }); setName(result.name); setSaved(true)
    } catch (error) { setError(error instanceof Error ? error.message : 'Не удалось сохранить название') }
    finally { submitting.current = false; setBusy(false) }
   }}>
    <label>Название семьи<input value={name} maxLength={80} required disabled={busy} onChange={event => { setName(event.target.value); setSaved(false) }} /></label>
    <button type="submit" disabled={busy || !name.trim() || name.trim() === value.name}>{busy ? 'Сохраняем…' : 'Сохранить название'}</button>
    {saved && <p role="status">Название семьи сохранено.</p>}
   </form>
   <p>Детские профили: {value.activeChildren} из {value.childLimit}</p>
   <p>{value.paid ? 'Оплачено' : value.access ? 'Предоставлен доступ' : 'Ожидает оплаты'}</p>
   <p>{value.accessUntil ? `Доступ до ${new Date(value.accessUntil).toLocaleString('ru-RU')}` : 'Срок доступа не ограничен'}</p>
   {!value.access && <p>История сохранена. Для продления обратитесь к оператору сервиса.</p>}
  </>}
 </section>
}
