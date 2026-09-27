import { useEffect, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import type { FamilySubscription as Subscription } from '../../domain/subscription'
export function FamilySubscription() {
 const {gateway}=useRuntime();const [value,setValue]=useState<Subscription|null>(null),[error,setError]=useState('')
 useEffect(()=>{let active=true;void gateway.subscription().then(v=>{if(active)setValue(v)}).catch(()=>{if(active)setError('Не удалось загрузить подписку')});return()=>{active=false}},[gateway])
 return <section className="panel"><h2>Семья и подписка</h2>{error&&<p role="alert">{error}</p>}{value&&<><h3>{value.name}</h3><p>Детские профили: {value.activeChildren} из {value.childLimit}</p><p>{value.paid?'Оплачено':value.access?'Предоставлен доступ':'Ожидает оплаты'}</p><p>{value.accessUntil?`Доступ до ${new Date(value.accessUntil).toLocaleString('ru-RU')}`:'Срок доступа не ограничен'}</p>{!value.access&&<p>История сохранена. Для продления обратитесь к оператору сервиса.</p>}</>}</section>
}
