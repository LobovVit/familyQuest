// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { MathWork } from './MathWork'
import type { MathView } from '../../domain/learning'
afterEach(cleanup)
const base: MathView = { id: 'test', finished: false, settings: { operation: '+', level: 'easy', answerMode: 'input', divisionMode: 'full', allowNegative: false }, index: 0, total: 8, correct: 0, stars: 0, bestStreak: 0, createdAt: '', question: { left: 2, right: 3, options: [] }, fields: [{ key: 'answer', label: 'Ответ', width: 1, shift: 0 }] }
it('submits numeric answers with the onscreen keypad', async () => {
 const submit = vi.fn(async () => {})
 render(<MathWork session={base} busy={false} onSubmit={submit} />)
 fireEvent.click(screen.getByRole('button', { name: '5' }))
 fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 await waitFor(() => expect(submit).toHaveBeenCalledWith({ answer: 5 }))
})
it('sends column carries and rejects missing results', async () => {
 const submit = vi.fn(async () => {})
 const session = { ...base, settings: { ...base.settings, level: 'columnar' as const }, question: { left: 58, right: 67, options: [] }, fields: [{ key: 'carry_0', label: 'Перенос единиц', width: 1, shift: 0 }, { key: 'carry_1', label: 'Перенос десятков', width: 1, shift: 0 }, { key: 'answer', label: 'Ответ', width: 3, shift: 0 }] }
 render(<MathWork session={session} busy={false} onSubmit={submit} />)
 fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 expect(submit).not.toHaveBeenCalled()
 expect(screen.getByRole('alert')).toBeTruthy()
 fireEvent.change(screen.getByLabelText('Перенос единиц'), { target: { value: '1' } })
 fireEvent.change(screen.getByLabelText('Перенос десятков'), { target: { value: '1' } })
 fireEvent.change(screen.getByLabelText('Ответ'), { target: { value: '125' } })
 fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 await waitFor(() => expect(submit).toHaveBeenCalledWith({ carry_0: 1, carry_1: 1, answer: 125 }))
})
it('supports negative answers and four-choice input', async () => {
 const submit = vi.fn(async () => {})
 render(<MathWork session={{ ...base, settings: { ...base.settings, operation: '-', allowNegative: true }, question: { left: 2, right: 5, options: [] } }} busy={false} onSubmit={submit} />)
 fireEvent.click(screen.getByRole('button', { name: '±' })); fireEvent.click(screen.getByRole('button', { name: '3' })); fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 await waitFor(() => expect(submit).toHaveBeenCalledWith({ answer: -3 }))
 cleanup(); submit.mockClear()
 render(<MathWork session={{ ...base, settings: { ...base.settings, answerMode: 'choice' }, question: { left: 2, right: 3, options: [2, 5, 6, 7] } }} busy={false} onSubmit={submit} />)
 fireEvent.click(screen.getByRole('button', { name: '5' }))
 await waitFor(() => expect(submit).toHaveBeenCalledWith({ answer: 5 }))
})
