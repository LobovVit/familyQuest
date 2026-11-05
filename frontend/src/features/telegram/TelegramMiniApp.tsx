import { useCallback, useEffect, useRef, useState } from 'react'
import type { TelegramGateway, TelegramHost, TelegramState } from '../../application/telegram'
import { useRuntime } from '../../application/runtime'
import { useSession } from '../../application/useSession'
import type { Participant } from '../../domain/models'
import { FamilyQuestWorkspace } from '../FamilyQuestWorkspace'
import type { WorkspaceSection } from '../navigation/sections'
import { telegramSection } from './navigation'
import './telegram.css'

export function TelegramMiniApp({client,host}:{client:TelegramGateway;host:TelegramHost}){
 const {session,gateway}=useRuntime(),{participant,login}=useSession()
 const [state,setState]=useState<TelegramState|null>(null),[profiles,setProfiles]=useState<Participant[]|null>(null)
 const [section,setSection]=useState<WorkspaceSection>(()=>telegramSection(host.startParam()))
 const [selected,setSelected]=useState(''),[pin,setPin]=useState(''),[linkCode,setLinkCode]=useState('')
 const [busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[closed,setClosed]=useState(false)
 const available=host.available()
 useEffect(()=>host.mount(),[host])
 useEffect(()=>{
  session.clearSession();if(!available)return
  let active=true
  void client.start().then(value=>{if(active)setState(value)}).catch(e=>{if(active)setError(e instanceof Error?e.message:'Не удалось войти через Telegram')})
  return ()=>{active=false}
 },[client,session,available])
 const refresh=useCallback(async()=>{
  try{const value=await client.state();setState(value);if(participant){const restored=await gateway.restoreSession();if(restored)login(restored);else{session.clearSession();setProfiles(null)}}}
  catch(e){session.clearSession();setProfiles(null);setState(null);setError(e instanceof Error?e.message:'Повторите вход в Telegram')}
 },[client,participant,gateway,login,session])
 useEffect(()=>{if(!state||closed)return;const check=()=>{void refresh()};window.addEventListener('focus',check);return()=>window.removeEventListener('focus',check)},[state,closed,refresh])
 const previousParticipant=useRef(participant)
 useEffect(()=>{const previous=previousParticipant.current;previousParticipant.current=participant;if(previous&&!participant){setProfiles(null);void refresh()}},[participant,refresh])
 const chooseFamily=useCallback(async(id:number)=>{
  setBusy(true);setError('');setProfiles(null);setSelected('');setPin('');session.clearSession()
  try{const result=await client.family(id);if(Array.isArray(result))setProfiles(result);else setState(result)}catch(e){setError(e instanceof Error?e.message:'Семья недоступна')}finally{setBusy(false)}
 },[client,session])
 const back=useCallback(()=>{if(participant&&section!=='day')setSection('day');else void chooseFamily(0)},[participant,section,chooseFamily])
 useEffect(()=>host.back(participant||profiles?back:null),[host,participant,profiles,back])
 async function act(action:()=>Promise<unknown>,message=''){
  if(busy)return;setBusy(true);setError('');setNotice('')
  try{await action();setNotice(message)}catch(e){setError(e instanceof Error?e.message:'Не удалось выполнить действие')}finally{setBusy(false)}
 }
 const webEntry=async()=>{try{await client.logout()}catch{/* A missing Telegram session must not trap normal web entry. */}session.clearSession();window.location.assign('/')}
 return <div className="telegram-app">
  <header className="mini-header"><strong>Семейные дела</strong>{state?.linked&&<a href={state.accountUrl} target="_blank" rel="noreferrer">Связь с Telegram</a>}{participant&&<button disabled={busy} onClick={()=>void chooseFamily(0)}>Выбрать семью</button>}<button onClick={()=>void act(webEntry)}>Обычный вход</button></header>
  {error&&<p className="notice" role="alert">{error}</p>}{notice&&<p role="status" className="notice">{notice}</p>}
  {!available?<section className="panel mini-entry"><h1>Откройте из Telegram</h1><p>Родительский кабинет доступен через кнопку приложения в боте. В браузере используйте обычный вход; там также можно создать свою семью.</p></section>:
   closed?<section className="panel mini-entry"><h1>Вы вышли</h1><p>Для нового входа закройте и заново откройте Mini App.</p></section>:
   !state?<section className="panel mini-entry"><p role="status">{error?'Закройте и заново откройте приложение для свежего входа.':'Проверяем Telegram…'}</p>{error&&<button disabled={busy} onClick={()=>void act(refresh)}>Повторить проверку</button>}</section>:
   !state.linked?<section className="panel mini-entry"><h1>Связать существующий аккаунт</h1><p>Откройте Делодень, войдите в свой аккаунт и получите код. Затем подтвердите там показанный Telegram ID.</p><a href={state.accountUrl} target="_blank" rel="noreferrer">Привязка в Делодне</a><form className="stack-form" onSubmit={e=>{e.preventDefault();void act(async()=>{await client.claim(linkCode);setLinkCode('')},'Код принят. Подтвердите связь в Делодне и нажмите «Проверить привязку».')}}><label>Код привязки<input value={linkCode} onChange={e=>setLinkCode(e.target.value)} autoComplete="off" maxLength={512}/></label><button disabled={busy||!linkCode}>Отправить код</button></form><button disabled={busy} onClick={()=>void act(refresh)}>Проверить привязку</button></section>:
   participant?<><div className="mini-actions">{participant.role==='parent'&&<button disabled={busy} onClick={()=>void act(async()=>{if(await host.requestWriteAccess()!==true)throw new Error('Разрешение Telegram на сообщения не получено. Уведомления не включены.');await client.consent(true)},'Согласие на уведомления сохранено. Если Telegram только что разрешил сообщения, при необходимости заново откройте Mini App.')}>Включить уведомления</button>}<button disabled={busy} onClick={()=>void act(()=>client.consent(false),'Уведомления выключены')}>Отключить уведомления</button><button disabled={busy} onClick={()=>void act(async()=>{await client.logout();session.clearSession();setProfiles(null);setState(null);setClosed(true)})}>Выйти</button></div><FamilyQuestWorkspace key={`${participant.familyId}:${participant.id}`} navigation={{section,setSection}} miniApp /></>:
   profiles?<section className="panel mini-entry"><h1>Выберите профиль</h1><p>Для родительских действий войдите в профиль взрослого. Ребёнку не нужен Telegram-аккаунт.</p><form className="stack-form" onSubmit={e=>{e.preventDefault();void act(async()=>{login(await client.profile(Number(selected),pin));setPin('')})}}><label>Профиль<select value={selected} onChange={e=>{setSelected(e.target.value);setPin('')}} required><option value="">Выберите профиль</option>{profiles.filter(p=>p.active).map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label><label>PIN<input type="password" inputMode="numeric" autoComplete="off" maxLength={6} value={pin} onChange={e=>setPin(e.target.value.replace(/\D/g,'').slice(0,6))}/></label><button disabled={busy||!selected||pin.length!==6}>Открыть профиль</button></form><button disabled={busy} onClick={()=>void chooseFamily(0)}>Назад к семьям</button></section>:
   <section className="panel mini-entry"><h1>Выберите свою семью</h1>{state.families.length?state.families.map(f=><button disabled={busy} key={f.id} onClick={()=>void chooseFamily(f.id)}>{f.name}</button>):<p>У этого аккаунта нет доступной семьи. В обычном веб-входе можно создать семью или восстановить доступ к существующей.</p>}</section>}
 </div>
}
