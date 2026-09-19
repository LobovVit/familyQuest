import { useMemo } from 'react'
import type { ChoreDraft, Task } from '../domain/models'
import type { ParticipantDraft, RewardDraft } from './ports'
import { useRuntime } from './runtime'

export function useFamilyQuestActions(refresh: () => Promise<void>) {
 const { gateway } = useRuntime()
 return useMemo(() => {
  const reload = async (operation: Promise<unknown>) => { await operation; await refresh() }
  return {
   login: gateway.login,
   saveChore: (id: number | 'new', draft: ChoreDraft) => reload(gateway.saveChore(id, draft)),
   completeTask: (task: Task) => reload(gateway.completeTask(task.id)),
   confirmTask: (task: Task, rating: number) => reload(gateway.confirmTask(task.id, rating)),
   rateBehavior: gateway.rateBehavior,
   createParticipant: (input: ParticipantDraft) => reload(gateway.createParticipant(input)),
   deleteParticipant: (id: number) => reload(gateway.deleteParticipant(id)),
   changePin: gateway.changePin,
   createReward: (input: RewardDraft) => reload(gateway.createReward(input)),
   deleteReward: (id: number) => reload(gateway.deleteReward(id)),
  }
 }, [gateway, refresh])
}
