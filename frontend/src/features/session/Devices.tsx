import { useEffect, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import type { TrustedDevice } from '../../domain/devices'
export function Devices({confirm,onCurrentRevoked}:{confirm:()=>Promise<string|null>;onCurrentRevoked:()=>void}){
 const {gateway}=useRuntime()
 const [items,setItems]=useState<TrustedDevice[]>([]),[error,setError]=useState(''),[loading,setLoading]=useState(true),[busy,setBusy]=useState(''),[attempt,setAttempt]=useState(0)
 useEffect(()=>{let active=true;setLoading(true);void gateway.devices().then(v=>{if(active){setItems(v);setError('')}}).catch(e=>{if(active)setError(e instanceof Error?e.message:'Не удалось загрузить устройства')}).finally(()=>{if(active)setLoading(false)});return()=>{active=false}},[gateway,attempt])
 async function revoke(d:TrustedDevice){
  if(busy)return
  setBusy(d.id);setError('')
  try{const proof=await confirm();if(!proof)return;await gateway.revokeDevice(d.id,proof);if(d.current){onCurrentRevoked();return};setItems(v=>v.filter(x=>x.id!==d.id))}
  catch(e){setError(e instanceof Error?e.message:'Не удалось отключить устройство')}
  finally{setBusy('')}
 }
 return <section className="panel"><h2>Устройства семьи</h2><p>Вход сохраняется на 90 дней и продлевается при использовании. Каждый браузер подключается отдельно. Смена PIN отключает устройства этого профиля.</p>{loading?<p role="status">Загружаем устройства…</p>:<>{error&&<p role="alert">{error}<button onClick={()=>setAttempt(v=>v+1)}>Повторить</button></p>}{!error&&!items.length&&<p>Пока нет запомненных устройств. При входе отметьте «Это моё устройство».</p>}<div className="family-grid">{items.map(d=><article className="family-card" key={d.id}><h3>{d.name}{d.current?' · это устройство':''}</h3><strong>{d.participantName}</strong><p>Последний вход: {new Date(d.lastSeenAt).toLocaleString('ru-RU')}</p><p>Вход сохранён до {new Date(d.expiresAt).toLocaleDateString('ru-RU')}</p><button disabled={!!busy} onClick={()=>{void revoke(d)}}>Отключить {d.name}</button></article>)}</div></>}</section>
}
