import { useCallback, useEffect, useRef, useState } from 'react'
import type { Assignment, BehaviorRating, Chore, LeaderboardEntry, Participant, Reward, Task } from '../domain/models'
import { useRuntime } from './runtime'

export function useFamilyQuestData(selectedDate: string, participant: Participant | null) {
 const { gateway, session } = useRuntime()
 const [participants, setParticipants] = useState<Participant[]>([])
 const [chores, setChores] = useState<Chore[]>([])
 const [assignments, setAssignments] = useState<Assignment[]>([])
 const [rewards, setRewards] = useState<Reward[]>([])
 const [tasks, setTasks] = useState<Task[]>([])
 const [dayLeaderboard, setDayLeaderboard] = useState<LeaderboardEntry[]>([])
 const [weekLeaderboard, setWeekLeaderboard] = useState<LeaderboardEntry[]>([])
 const [monthLeaderboard, setMonthLeaderboard] = useState<LeaderboardEntry[]>([])
 const [behaviorRatings, setBehaviorRatings] = useState<BehaviorRating[]>([])
 const [isLoading, setIsLoading] = useState(true)
 const [loadError, setLoadError] = useState('')
 const activeDate = useRef(selectedDate)
 const generation = useRef(0)
 const participantGeneration = useRef(0)
 const clearProtected = useCallback(() => {
  setChores([]); setAssignments([]); setRewards([]); setTasks([]); setBehaviorRatings([])
 }, [])
 const loadParticipants = useCallback(async () => {
  const request = ++participantGeneration.current
  try {
   const values = await gateway.participants()
   if (request === participantGeneration.current) setParticipants(values)
  } catch (error) {
   if (request === participantGeneration.current) setLoadError(error instanceof Error ? error.message : 'Не удалось загрузить участников')
  }
 }, [gateway])
 const refresh = useCallback(async () => {
  // Old mutation callbacks must not invalidate or reload the new view.
  if (session.getParticipant() !== participant || activeDate.current !== selectedDate) return
  const request = ++generation.current
  const current = () => request === generation.current && session.getParticipant() === participant && activeDate.current === selectedDate
  setIsLoading(true)
  setLoadError('')
  if (!participant) clearProtected()
  try {
   const [day, week, month, protectedData] = await Promise.all([
    gateway.leaderboard('day', selectedDate), gateway.leaderboard('week', selectedDate), gateway.leaderboard('month', selectedDate),
    participant ? Promise.all([gateway.chores(), gateway.assignments(), gateway.rewards(), gateway.tasks(selectedDate), gateway.ratings(selectedDate)] as const) : null,
   ])
   if (!current()) return
   setDayLeaderboard(day); setWeekLeaderboard(week); setMonthLeaderboard(month)
   if (protectedData) {
    setChores(protectedData[0]); setAssignments(protectedData[1]); setRewards(protectedData[2]); setTasks(protectedData[3]); setBehaviorRatings(protectedData[4])
   }
  } catch (error) {
   if (current()) setLoadError(error instanceof Error ? error.message : 'Не удалось загрузить данные')
  } finally { if (current()) setIsLoading(false) }
 }, [gateway, session, participant, selectedDate, clearProtected])
 const invalidateParticipants = useCallback(() => { ++participantGeneration.current }, [])
 const invalidateData = useCallback(() => { ++generation.current }, [])
 useEffect(() => {
  void loadParticipants()
  return invalidateParticipants
 }, [loadParticipants, invalidateParticipants])
 useEffect(() => {
  activeDate.current = selectedDate
  clearProtected()
  void refresh()
  return invalidateData
 }, [refresh, clearProtected, invalidateData, selectedDate])
 return { participants, chores, assignments, rewards, tasks, dayLeaderboard, weekLeaderboard, monthLeaderboard,
  behaviorRatings, isLoading, loadError, refresh, loadParticipants }
}
