import { describe, expect, it } from 'vitest'
import { canCompleteTask, canReviewTask, taskProgress } from './policies'
import type { Participant, Task } from './models'

const parent: Participant = { id: 1, name: 'Adult', role: 'parent', active: true }
const task = { id: 1, participantId: 2, status: 'completed' } as Task

describe('task policies', () => {
  it('allows only the assignee to complete a task', () => {
    expect(canCompleteTask({ ...parent, id: 2 }, task)).toBe(true)
    expect(canCompleteTask(parent, task)).toBe(false)
  })
  it('allows a different parent to review a completed task', () => {
    expect(canReviewTask(parent, task)).toBe(true)
    expect(canReviewTask({ ...parent, role: 'child' }, task)).toBe(false)
  })
  it('calculates progress without division by zero', () => {
    expect(taskProgress([])).toBe(0)
    expect(taskProgress([task, { ...task, id: 2, status: 'pending' }])).toBe(50)
  })
})
