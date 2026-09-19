import { createContext, useContext } from 'react'
import type { Runtime } from './ports'
export const RuntimeContext = createContext<Runtime | null>(null)
export function useRuntime(): Runtime {
 const runtime = useContext(RuntimeContext)
 if (!runtime) throw new Error('FamilyQuest runtime is not configured')
 return runtime
}
