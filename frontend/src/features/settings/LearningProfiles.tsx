import { useEffect, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import type { Participant } from '../../domain/models'
import type { LearningProfile } from '../../domain/subscription'
import { mathLevels } from '../../domain/learning'
import { readingModes } from '../../domain/reading'
import { localDate } from '../../domain/date'
export function LearningProfiles({participants}:{participants:Participant[]}) {
 return <section className="users-grid" aria-label="Возраст и обучение"><h2>Возраст и обучение детей</h2>{participants.filter(p=>p.role==='child').map(p=><Profile key={p.id} person={p}/>)}</section>
}
function Profile({person}:{person:Participant}) {
 const {gateway}=useRuntime();const [value,setValue]=useState<LearningProfile|null>(null),[message,setMessage]=useState(''),[busy,setBusy]=useState(false)
 useEffect(()=>{let active=true;void gateway.learningProfile(person.id).then(v=>{if(active)setValue(v)}).catch(()=>{if(active)setMessage('Не удалось загрузить настройки')});return()=>{active=false}},[gateway,person.id])
 return <form className="panel stack-form" onSubmit={async e=>{e.preventDefault();if(!value||busy)return;setBusy(true);setMessage('');try{await gateway.saveLearningProfile(person.id,value);setMessage('Сохранено. Применится к новым занятиям.')}catch(e){setMessage(e instanceof Error?e.message:'Не удалось сохранить')}finally{setBusy(false)}}}><h3>{person.name}</h3>{value&&<><label>Дата рождения<input type="date" max={localDate(new Date())} value={value.birthDate} onInput={e=>setValue({...value,birthDate:e.currentTarget.value})} onChange={e=>setValue({...value,birthDate:e.target.value})}/></label><label>Математика<select value={value.mathLevel} onChange={e=>setValue({...value,mathLevel:e.target.value as LearningProfile['mathLevel']})}><option value="">По возрасту</option>{Object.entries(mathLevels).map(([key,label])=><option key={key} value={key}>{label}</option>)}</select></label><label>Чтение<select value={value.readingLevel} onChange={e=>setValue({...value,readingLevel:e.target.value as LearningProfile['readingLevel']})}><option value="">По возрасту</option>{readingModes.map(m=><option key={m.id} value={m.id}>{m.title}</option>)}</select></label><button disabled={busy}>Сохранить настройки</button></>}{message&&<p role="status">{message}</p>}</form>
}
