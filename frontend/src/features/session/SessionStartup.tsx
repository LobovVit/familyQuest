import { useEffect, useState, type ReactNode } from 'react'
export function SessionStartup({initialize,refresh,children}:{initialize:()=>Promise<void>;refresh:()=>Promise<void>;children:ReactNode}){
 const [ready,setReady]=useState(false),[error,setError]=useState('')
 useEffect(()=>{let active=true;void initialize().then(()=>{if(active)setReady(true)}).catch(()=>{if(active)setError('Не удалось проверить вход. Проверьте подключение и повторите.')});return()=>{active=false}},[initialize])
 useEffect(()=>{
  if(!ready)return
  const check=()=>{void refresh().catch(()=>{/* Existing view handles network failures on its next request. */})}
  window.addEventListener('focus',check);return()=>window.removeEventListener('focus',check)
 },[ready,refresh])
 if(!ready)return <main className="panel"><h1>FamilyQuest</h1>{error?<><p role="alert">{error}</p><button onClick={()=>window.location.reload()}>Повторить</button></>:<p role="status">Проверяем вход…</p>}</main>
 return children
}
