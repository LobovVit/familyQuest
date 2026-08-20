import { useCallback, useEffect, useState } from 'react'
import type { Assignment, BehaviorRating, Chore, LeaderboardEntry, Participant, Reward, Task } from '../domain/models'
import { api } from '../infrastructure/apiClient'

export function useFamilyQuestData(selectedDate: string, authenticated: boolean) {
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

  const loadParticipants = useCallback(async () => {
    try { setParticipants(await api<Participant[]>('/api/participants')) }
    catch (error) { setLoadError(error instanceof Error ? error.message : 'Не удалось загрузить участников') }
  }, [])
  const clearProtected = useCallback(() => {
    setChores([]); setAssignments([]); setRewards([]); setTasks([]); setDayLeaderboard([]); setWeekLeaderboard([]); setMonthLeaderboard([]); setBehaviorRatings([])
  }, [])
  const refresh = useCallback(async () => {
    if (!authenticated) { clearProtected(); setIsLoading(false); return }
    setLoadError('')
    try {
      const values = await Promise.all([
        api<Chore[]>('/api/chores'), api<Assignment[]>('/api/assignments'), api<Reward[]>('/api/rewards'), api<Task[]>(`/api/tasks?date=${selectedDate}`),
        api<LeaderboardEntry[]>(`/api/leaderboard?period=day&date=${selectedDate}`),
        api<LeaderboardEntry[]>(`/api/leaderboard?period=week&date=${selectedDate}`),
        api<LeaderboardEntry[]>(`/api/leaderboard?period=month&date=${selectedDate}`),
        api<BehaviorRating[]>(`/api/behavior-ratings?date=${selectedDate}`),
      ] as const)
      setChores(values[0]); setAssignments(values[1]); setRewards(values[2]); setTasks(values[3])
      setDayLeaderboard(values[4]); setWeekLeaderboard(values[5]); setMonthLeaderboard(values[6]); setBehaviorRatings(values[7])
    } catch (error) { setLoadError(error instanceof Error ? error.message : 'Не удалось загрузить данные') }
    finally { setIsLoading(false) }
  }, [authenticated, clearProtected, selectedDate])

  useEffect(() => { void loadParticipants() }, [loadParticipants])
  useEffect(() => { void refresh() }, [refresh])
  return { participants, chores, assignments, rewards, tasks, dayLeaderboard, weekLeaderboard, monthLeaderboard,
    behaviorRatings, setBehaviorRatings, isLoading, loadError, refresh, loadParticipants }
}
