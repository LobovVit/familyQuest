// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { Task } from '../../domain/models'
import { MyDay } from './MyDay'
afterEach(cleanup)
const participant = { id: 3, name: 'Макс', role: 'child' as const, active: true }
const task = (id: number, status: Task['status'], owner = 3): Task => ({ id, participantId: owner, assignmentId: id, choreTitle: `Дело ${id}`, choreDescription: '', personName: 'Макс', dueDate: '2026-09-23', schedule: 'daily', timeWindow: 'morning', benefitType: 'self', executionMode: 'assigned', status, averageRating: 0, reward: 0 })
it('counts only own completed work, keeps needs_work actionable and limits the first steps', () => {
 const onComplete = vi.fn(), onNavigate = vi.fn()
 render(<MyDay participant={participant} tasks={[task(1, 'completed'), task(2, 'needs_work'), task(3, 'pending'), task(4, 'pending'), task(5, 'pending'), task(6, 'confirmed', 1)]} assignments={[]} loading={false} busyTask={null} reviewCount={0} onComplete={onComplete} onNavigate={onNavigate} />)
 expect(screen.getByText('1 из 5')).toBeTruthy()
 expect(screen.getByRole('progressbar').getAttribute('value')).toBe('20')
 expect(screen.getByText('Попробуем ещё раз')).toBeTruthy()
 expect(screen.queryByText('Дело 6')).toBeNull()
 expect(screen.queryByText('Дело 5')).toBeNull()
 fireEvent.click(screen.getAllByRole('button', { name: '✓ Сделано' })[0])
 expect(onComplete.mock.calls[0][0].id).toBe(2)
 fireEvent.click(screen.getByRole('button', { name: 'Ещё дел в планере: 1 →' }))
 expect(onNavigate).toHaveBeenCalledWith('day')
})
it('distinguishes pending confirmation from an earned reward and preserves parent navigation', () => {
 const onNavigate = vi.fn()
 render(<MyDay participant={{ ...participant, role: 'parent' }} tasks={[task(1, 'completed')]} assignments={[]} loading={false} busyTask={null} reviewCount={2} onComplete={vi.fn()} onNavigate={onNavigate} />)
 expect(screen.getByText('Отметки сохранены. Звёзды за дела появятся после подтверждения.')).toBeTruthy()
 expect(screen.queryByText('Поиграем с числами')).toBeNull()
 fireEvent.click(screen.getByRole('button', { name: /Дела семьи ждут/ }))
 expect(onNavigate).toHaveBeenCalledWith('day')
})
