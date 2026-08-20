import type { Participant, Task } from './models'

export const isParent = (participant: Participant | null): boolean => participant?.role === 'parent'
export const canCompleteTask = (participant: Participant | null, task: Task): boolean => participant?.id === task.participantId
export const canReviewTask = (participant: Participant | null, task: Task): boolean =>
  isParent(participant) && participant?.id !== task.participantId && task.status === 'completed'
export const taskProgress = (tasks: Task[]): number => {
  if (tasks.length === 0) return 0
  return Math.round(tasks.filter((task) => task.status !== 'pending').length / tasks.length * 100)
}
