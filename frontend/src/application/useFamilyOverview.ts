import { useEffect, useState } from 'react'
import { useRuntime } from './runtime'
import type { FamilyOverview } from '../domain/family'
export function useFamilyOverview(date: string) {
  const { gateway } = useRuntime()
  const [state, setState] = useState<{ date: string; data?: FamilyOverview; error?: string }>({ date })
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    let active = true
    setState({ date })
    void gateway.familyOverview(date).then(data => {
      if (active) setState({ date, data })
    }).catch(error => {
      if (active) setState({ date, error: error instanceof Error ? error.message : 'Не удалось загрузить семейный обзор' })
    })
    return () => { active = false }
  }, [gateway, date, attempt])
  return { data: state.date === date ? state.data : undefined, error: state.date === date ? state.error : undefined, retry: () => setAttempt(a => a + 1) }
}
