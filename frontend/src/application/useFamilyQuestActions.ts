import { useCallback } from 'react'
import type { BehaviorRating, ChoreDraft, LoginResponse, Participant, RewardPeriod, RewardType, Task } from '../domain/models'
import { api } from '../infrastructure/apiClient'

export function useFamilyQuestActions(refresh: () => Promise<void>) {
  const login = useCallback((participantId:number, pin:string) => api<LoginResponse>('/api/session', { method:'POST', body:JSON.stringify({ participantId, pin }) }), [])
  const saveChore = useCallback(async (id:number|'new', draft:ChoreDraft) => { await api(id === 'new' ? '/api/chores' : `/api/chores/${id}`, { method:id === 'new' ? 'POST':'PUT', body:JSON.stringify(draft) }); await refresh() }, [refresh])
  const completeTask = useCallback(async (task:Task, participantId:number) => { await api(`/api/tasks/${task.id}/complete`, { method:'POST', body:JSON.stringify({ participantId }) }); await refresh() }, [refresh])
  const confirmTask = useCallback(async (task:Task, participantId:number, rating:number) => { await api(`/api/tasks/${task.id}/confirm`, { method:'POST', body:JSON.stringify({ participantId, rating }) }); await refresh() }, [refresh])
  const rateBehavior = useCallback((date:string, raterParticipantId:number, targetParticipantId:number, rating:number) => api<BehaviorRating>('/api/behavior-ratings', { method:'POST', body:JSON.stringify({ date, raterParticipantId, targetParticipantId, rating, comment:'' }) }), [])
  const createParticipant = useCallback(async (input:{name:string;role:Participant['role'];pin:string}) => { await api('/api/participants', { method:'POST', body:JSON.stringify(input) }); await refresh() }, [refresh])
  const deleteParticipant = useCallback(async (id:number) => { await api(`/api/participants/${id}`, {method:'DELETE'}); await refresh() }, [refresh])
  const changePin = useCallback((id:number,pin:string) => api(`/api/participants/${id}/pin`, {method:'PUT',body:JSON.stringify({pin})}), [])
  const createReward = useCallback(async (input:{title:string;description:string;period:RewardPeriod;rewardType:RewardType;starCost:number;smileCost:number;participantIds:number[]}) => { await api('/api/rewards',{method:'POST',body:JSON.stringify(input)}); await refresh() }, [refresh])
  const deleteReward = useCallback(async (id:number) => { await api(`/api/rewards/${id}`,{method:'DELETE'}); await refresh() }, [refresh])
  return { login, saveChore, completeTask, confirmTask, rateBehavior, createParticipant, deleteParticipant, changePin, createReward, deleteReward }
}
