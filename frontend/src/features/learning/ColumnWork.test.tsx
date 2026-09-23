// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { MathView } from '../../domain/learning'
import { ColumnWork } from './ColumnWork'
import { columnLayout } from './columnLayout'
afterEach(cleanup)
function session(op: '+' | '-' | '*' | ':', left: number, right: number, expected: Record<string, number>, mode: 'result' | 'steps' | 'full' = 'full'): MathView {
 return { id: 'test', finished: false, settings: { operation: op, level: 'columnar', answerMode: 'input', divisionMode: mode, allowNegative: false }, index: 0, total: 8, correct: 0, stars: 0, bestStreak: 0, createdAt: '', question: { left, right, options: [] }, fields: Object.keys(expected).map(key => ({ key, label: key, width: 4, shift: 0 })) }
}
async function solve(view: MathView, expected: Record<string, number>) {
 const submit = vi.fn(async () => {})
 render(<ColumnWork session={view} busy={false} onSubmit={submit} />)
 const layout = columnLayout(view)
 for (const id of layout.order) {
  const c = layout.cells.find(c => c.id === id)!
  const digit = String(expected[c.key]).padStart(layout.widths[c.key], '0')[c.digit]
  fireEvent.focus(screen.getByLabelText(c.label))
  fireEvent.click(screen.getByRole('button', { name: digit }))
 }
 fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 await waitFor(() => expect(submit).toHaveBeenCalledWith(expected))
}
it('places shifted multiplication rows and enters units before tens', async () => {
 const expected = { mulcarry_0_0: 2, mulcarry_0_1: 2, mulcarry_0_2: 1, partial_0: 1404, mulcarry_1_0: 2, mulcarry_1_1: 1, mulcarry_1_2: 1, partial_1: 1170, answer: 13104 }
 const view = session('*', 234, 56, expected), layout = columnLayout(view)
 expect(layout.order.slice(0, 4)).toEqual(['partial_0:3', 'mulcarry_0_0:0', 'partial_0:2', 'mulcarry_0_1:0'])
 const units = layout.cells.find(c => c.id === 'partial_0:3')!, tens = layout.cells.find(c => c.id === 'partial_1:3')!
 expect(tens.col).toBe(units.col - 1); expect(tens.row).toBe(units.row + 3)
 await solve(view, expected)
})
it('uses a single product as the answer for a one-digit multiplier', async () => {
 const expected = { mulcarry_0_0: 2, mulcarry_0_1: 4, partial_0: 432, answer: 432 }
 await solve(session('*', 72, 6, expected), expected)
})
it('keeps zero in the quotient and brings down exactly one digit', async () => {
 const expected = { product_0: 6, remainder_0: 0, bring_1: 5, product_1: 0, remainder_1: 5, bring_2: 4, product_2: 54, remainder_2: 0, answer: 109 }
 const view = session(':', 654, 6, expected)
 const layout = columnLayout(view)
 expect(layout.order.slice(0, 6)).toEqual(['answer:0', 'product_0:0', 'remainder_0:0', 'bring_1:0', 'answer:1', 'product_1:0'])
 await solve(view, expected)
})
it('supports first partial spanning two digits and all division modes', async () => {
 for (const mode of ['result', 'steps', 'full'] as const) {
  const expected: Record<string, number> = { answer: 69 }
  if (mode !== 'result') Object.assign(expected, { product_0: 24, remainder_0: 3, product_1: 36, remainder_1: 0 })
  if (mode === 'full') expected.bring_1 = 6
  const view = session(':', 276, 4, expected, mode)
  const layout = columnLayout(view)
  if (mode !== 'result') expect(layout.cells.find(c => c.id === 'product_0:1')?.col).toBe(2)
  await solve(view, expected); cleanup()
 }
})
it('rejects missing digits and blocks submission while busy', () => {
 const submit = vi.fn(async () => {}), view = session(':', 840, 6, { answer: 140 }, 'result')
 const { rerender } = render(<ColumnWork session={view} busy={false} onSubmit={submit} />)
 fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 expect(screen.getByRole('alert')).toBeTruthy(); expect(submit).not.toHaveBeenCalled()
 rerender(<ColumnWork session={view} busy onSubmit={submit} />)
 expect(screen.getByRole('button', { name: '0' }).closest('fieldset')?.disabled).toBe(true)
})
it('preserves a trailing zero in the quotient', async () => {
 const expected = { product_0: 6, remainder_0: 2, bring_1: 4, product_1: 24, remainder_1: 0, bring_2: 0, product_2: 0, remainder_2: 0, answer: 140 }
 await solve(session(':', 840, 6, expected), expected)
})
it('submits the child digits without replacing mistakes with calculated answers', async () => {
 const view = session(':', 12, 3, { answer: 4 }, 'result')
 await solve(view, { answer: 5 })
})

it('toggles carry dots without moving the selected answer digit', async () => {
 const submit = vi.fn(async () => {})
 render(<ColumnWork session={session('+', 58, 67, { carry_0: 1, carry_1: 1, answer: 125 })} busy={false} onSubmit={submit} />)
 const dot = screen.getByRole('button', { name: 'carry_0' })
 fireEvent.click(dot); expect(dot.getAttribute('aria-pressed')).toBe('true')
 fireEvent.click(dot); expect(dot.getAttribute('aria-pressed')).toBe('false')
 fireEvent.click(dot)
 expect((screen.getByRole('button', { name: 'carry_1' }) as HTMLButtonElement).disabled).toBe(true)
 fireEvent.click(screen.getByRole('button', { name: '5' }))
 expect((screen.getByLabelText('answer, разряд 1') as HTMLInputElement).className).toContain('is-selected')
 fireEvent.focus(screen.getByLabelText('answer, разряд 2'))
 fireEvent.click(screen.getByRole('button', { name: 'carry_1' }))
 fireEvent.click(screen.getByRole('button', { name: '2' }))
 fireEvent.focus(screen.getByLabelText('answer, разряд 3'))
 fireEvent.click(screen.getByRole('button', { name: '1' }))
 expect((screen.getByLabelText('answer, разряд 1') as HTMLInputElement).value).toBe('5')
 fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 await waitFor(() => expect(submit).toHaveBeenCalledWith({ carry_0: 1, carry_1: 1, answer: 125 }))
})
it('represents borrowing through zero with dots over the next places', async () => {
 const submit = vi.fn(async () => {})
 render(<ColumnWork session={session('-', 300, 127, { carry_0: 1, carry_1: 1, carry_2: 0, answer: 173 })} busy={false} onSubmit={submit} />)
 expect(screen.queryByRole('button', { name: 'carry_2' })).toBeNull()
 fireEvent.click(screen.getByRole('button', { name: 'carry_0' }))
 fireEvent.click(screen.getByRole('button', { name: '3' }))
 fireEvent.focus(screen.getByLabelText('answer, разряд 2'))
 fireEvent.click(screen.getByRole('button', { name: 'carry_1' }))
 fireEvent.click(screen.getByRole('button', { name: '7' }))
 fireEvent.focus(screen.getByLabelText('answer, разряд 3'))
 fireEvent.click(screen.getByRole('button', { name: '1' }))
 fireEvent.click(screen.getByRole('button', { name: 'Проверить' }))
 await waitFor(() => expect(submit).toHaveBeenCalledWith({ carry_0: 1, carry_1: 1, carry_2: 0, answer: 173 }))
})
it('rejects multi-digit paste and replaces a selected digit without concatenation', () => {
 render(<ColumnWork session={session('+', 58, 67, { carry_0: 1, carry_1: 1, answer: 125 })} busy={false} onSubmit={async () => {}} />)
 const units = screen.getByLabelText('answer, разряд 1') as HTMLInputElement
 fireEvent.paste(units, { clipboardData: { getData: () => '125' } }); expect(units.value).toBe('')
 fireEvent.keyDown(units, { key: '5' }); expect(units.value).toBe('5')
 fireEvent.focus(units); fireEvent.keyDown(units, { key: '6' }); expect(units.value).toBe('6')
 fireEvent.click(screen.getByRole('button', { name: 'Очистить' })); expect(units.value).toBe('')
})
it('creates one partial per multiplier digit including zero and addition only when needed', () => {
 for (const right of [6, 56, 206]) {
  const expected: Record<string, number> = { answer: 234 * right }
  for (let i = 0; i < String(right).length; i++) expected[`partial_${i}`] = 0
  const layout = columnLayout(session('*', 234, right, expected))
  const rows = new Set(layout.cells.filter(c => c.key.startsWith('partial_')).map(c => c.row))
  expect(rows.size).toBe(String(right).length)
  expect(layout.cells.some(c => c.key === 'answer')).toBe(right >= 10)
  expect(layout.cells.some(c => c.memo)).toBe(right >= 10)
  if (right === 206) expect(layout.cells.filter(c => c.key === 'partial_1')).toHaveLength(1)
 }
})
it('toggles final addition memos as dots without changing the selected answer or numeric multiplication carries', () => {
 render(<ColumnWork session={session('*', 24, 13, { partial_0: 72, mulcarry_0_0: 1, mulcarry_0_1: 0, partial_1: 24, answer: 312 })} busy={false} onSubmit={async () => {}} />)
 const memo = screen.getByRole('button', { name: 'Сложение строк: в уме из разряда 1' }) as HTMLButtonElement
 expect(memo.disabled).toBe(true)
 expect(screen.queryByRole('textbox', { name: 'Сложение строк: в уме из разряда 1' })).toBeNull()
 expect(screen.getByRole('textbox', { name: 'mulcarry_0_0, разряд 1' })).toBeTruthy()
 const answer = screen.getByLabelText('answer, разряд 1') as HTMLInputElement
 fireEvent.focus(answer)
 expect(memo.disabled).toBe(false)
 fireEvent.click(memo); expect(memo.getAttribute('aria-pressed')).toBe('true')
 fireEvent.click(memo); expect(memo.getAttribute('aria-pressed')).toBe('false')
 fireEvent.click(memo)
 fireEvent.click(screen.getByRole('button', { name: '2' }))
 expect(answer.value).toBe('2')
 fireEvent.click(screen.getByRole('button', { name: 'Очистить' }))
 expect(memo.getAttribute('aria-pressed')).toBe('false'); expect(answer.value).toBe('')
})

it('highlights the operand places and keeps carry selection independent from product digits', () => {
 const { container } = render(<ColumnWork session={session('*', 24, 13, { partial_0: 72, mulcarry_0_0: 1, mulcarry_0_1: 0, partial_1: 24, answer: 312 })} busy={false} onSubmit={async () => {}} />)
 expect([...container.querySelectorAll('.column-active-operand')].map(el => el.textContent)).toEqual(['4', '3'])
 fireEvent.click(screen.getByRole('button', { name: '2' }))
 const units = screen.getByLabelText('partial_0, разряд 1') as HTMLInputElement
 expect(units.value).toBe('2'); expect(units.className).toContain('is-selected')
 fireEvent.focus(screen.getByLabelText('mulcarry_0_0, разряд 1'))
 fireEvent.click(screen.getByRole('button', { name: '1' }))
 expect(units.value).toBe('2')
 fireEvent.focus(screen.getByLabelText('partial_1, разряд 1'))
 expect([...container.querySelectorAll('.column-active-operand')].map(el => el.textContent)).toEqual(['4', '1'])
})

it('matches the multiplication reference: one carry between digits, shifted zero and separate signs', () => {
 const layout = columnLayout(session('*', 87, 25, { partial_0: 435, mulcarry_0_0: 3, mulcarry_0_1: 4, partial_1: 174, mulcarry_1_0: 1, mulcarry_1_1: 1, answer: 2175 }))
 expect(layout.cells.filter(c => c.key.startsWith('mulcarry_')).map(c => c.key)).toEqual(['mulcarry_0_0', 'mulcarry_1_0'])
 expect(layout.marks.filter(m => m.kind === 'sign').map(m => m.text)).toEqual(['+', '='])
 expect(layout.marks.find(m => m.kind === 'shift')).toMatchObject({ text: '0', col: layout.size })
 expect(layout.marks.some(m => m.kind === 'empty')).toBe(true)
})
