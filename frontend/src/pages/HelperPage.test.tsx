// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { Runtime } from '../application/ports'
import { RuntimeContext } from '../application/runtime'
import { HelperPage } from './HelperPage'

afterEach(cleanup)

it('clears visible tasks when the same open helper session loses access', async () => {
 const helperTasks = vi.fn()
  .mockResolvedValueOnce([{title: 'Полить растение', date: '2026-11-27'}])
  .mockRejectedValueOnce(new Error('Access revoked'))
 const runtime = {gateway: {helperTasks}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><HelperPage /></RuntimeContext.Provider>)

 fireEvent.change(screen.getByLabelText('Код доступа'), {target: {value: 'synthetic-code'}})
 fireEvent.click(screen.getByRole('button', {name: 'Открыть дела'}))
 await screen.findByText('Полить растение')
 expect(screen.queryByLabelText('Код доступа')).toBeNull()

 fireEvent(window, new Event('focus'))
 await screen.findByText('Доступ недоступен. Проверьте код или свяжитесь с родителем.')
 expect(screen.queryByText('Полить растение')).toBeNull()
 expect(helperTasks.mock.calls.map(([code]) => code)).toEqual(['synthetic-code', 'synthetic-code'])

 fireEvent.click(screen.getByRole('button', {name: 'Выйти'}))
 expect((screen.getByLabelText('Код доступа') as HTMLInputElement).value).toBe('')
})
