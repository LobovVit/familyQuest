import { useEffect, useState } from 'react'
import { FamilyQuestWorkspace } from '../features/FamilyQuestWorkspace'
import { AccountLogin } from '../features/session/AccountLogin'
import { useRuntime } from '../application/runtime'
import { useSession } from '../application/useSession'
export function FamilyQuestPage() {
 const { gateway } = useRuntime(), { participant } = useSession()
 const [config, setConfig] = useState<{saas:boolean} | null>(null), [error,setError] = useState(''), [attempt,setAttempt] = useState(0)
 useEffect(() => {let active=true;setError('');void gateway.config().then(v=>{if(active)setConfig(v)}).catch(()=>{if(active)setError('Не удалось подключиться к сервису')});return()=>{active=false}},[gateway,attempt])
 if (!config) return <main className="account-entry"><p role="status">{error || 'Подключаемся…'}</p>{error && <button onClick={()=>setAttempt(v=>v+1)}>Повторить</button>}</main>
 if (config.saas && !participant) return <AccountLogin />
 return <FamilyQuestWorkspace key={`${participant?.familyId ?? 0}:${participant?.id ?? 0}`} />
}
