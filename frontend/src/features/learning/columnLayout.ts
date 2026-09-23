import type { MathView } from '../../domain/learning'

export type ColumnCell = { id: string; key: string; digit: number; label: string; row: number; col: number; carry?: boolean; dot?: boolean; memo?: boolean }
type Mark = { text: string; row: number; col: number; width?: number; line?: boolean; operand?: 'top' | 'bottom'; kind?: 'shift' | 'empty' | 'sign' }

// RU: Координаты — разряды тетради; порядок ввода хранится отдельно.
// EN: Coordinates represent notebook places; input order is independent.
export function columnLayout(session: MathView) {
 const { left, right } = session.question!
 const operation = session.settings.operation
 const division = operation === ':'
 const multiplication = operation === '*'
 const arithmetic = operation === '+' || operation === '-'
 const cells: ColumnCell[] = [], marks: Mark[] = [], order: string[] = []
 const widths: Record<string, number> = {}
 const leftDigits = String(left).length
 const result = operation === '+' ? left + right : operation === '-' ? left - right : left * right
 const size = division ? leftDigits : arithmetic ? Math.max(leftDigits, String(right).length, String(result).length) + 1 : Math.max(leftDigits + String(right).length, String(result).length) + 1
 const addNumber = (text: string, row: number, end: number, operand?: 'top' | 'bottom') => [...text].forEach((digit, i) => marks.push({ text: digit, row, col: end - text.length + 1 + i, operand }))
 const addField = (key: string, width: number, row: number, end: number, reverse = false, carry = false) => {
  const field = session.fields.find(f => f.key === key)
  if (!field) return []
  widths[key] = width
  const added: ColumnCell[] = Array.from({ length: width }, (_, digit) => ({ id: `${key}:${digit}`, key, digit, label: `${field.label}, разряд ${width - digit}`, row, col: end - width + 1 + digit, carry }))
  cells.push(...added)
  const ids = added.map(c => c.id)
  order.push(...(reverse ? ids.reverse() : ids))
  return added
 }
 let rows = 3
 if (arithmetic) {
  addNumber(String(left), 2, size, 'top'); addNumber(String(right), 3, size, 'bottom')
  marks.push({ text: operation === '+' ? '+' : '−', row: 3, col: 1 }, { text: '', row: 3, col: 2, width: size - 1, line: true })
  for (let j = 0; j < size - 2; j++) {
   const added = addField(`carry_${j}`, 1, 1, size - j - 1, false, true)
   for (const c of added) { c.dot = true; c.label = session.fields.find(f => f.key === c.key)!.label }
  }
  order.length = 0
  addField('answer', String(result).length, 4, size, true)
  rows = 4
 } else if (multiplication) {
  const multipliers = [...String(right)].reverse()
  const operandRow = 1
  addNumber(String(left), operandRow, size, 'top'); addNumber(String(right), operandRow + 1, size, 'bottom')
  marks.push({ text: '×', row: operandRow + 1, col: 1 }, { text: '', row: operandRow + 1, col: 2, width: size - 1, line: true })
  multipliers.forEach((digit, i) => {
   const row = operandRow + 3 + i * 3
   if (i > 0) marks.push({ text: '+', row: row - 2, col: 1, kind: 'sign' })
   for (let shift = 0; shift < i; shift++) marks.push({ text: '0', row, col: size - shift, kind: 'shift' })
   const partialKey = `partial_${i}`
   const partial = addField(partialKey, String(left * Number(digit)).length, row, size - i, true)
   for (let col = 2; col < size - i - partial.length + 1; col++) marks.push({ text: '', row, col, kind: 'empty' })
   const partialOrder = order.splice(order.length - partial.length)
   for (let j = 0; j < leftDigits; j++) {
    if (partialOrder[j]) order.push(partialOrder[j])
    if (j < leftDigits - 1) addField(`mulcarry_${i}_${j}`, 1, row - 1, size - i - j - 1, false, true)
   }
   order.push(...partialOrder.slice(leftDigits))
   rows = row
  })
  if (multipliers.length > 1) {
   marks.push({ text: '=', row: rows + 1, col: 1, kind: 'sign' }, { text: '', row: rows, col: 2, width: size - 1, line: true })
   for (let j = 0; j < String(left * right).length - 1; j++) {
    const key = `sumcarry_${j}`
    widths[key] = 1
    cells.push({ id: `${key}:0`, key, digit: 0, label: `Сложение строк: в уме из разряда ${j + 1}`, row: rows + 1, col: size - j - 1, carry: true, memo: true })
   }
   addField('answer', String(left * right).length, rows + 2, size, true)
   rows += 2
  }
 } else {
  addNumber(String(left), 1, size)
  addNumber(String(right), 1, size + String(right).length + 1)
  const quotient = String(Math.floor(left / right))
  const answerCells = addField('answer', quotient.length, 2, size + 1 + quotient.length)
  order.length = 0
  if (session.settings.divisionMode === 'result') order.push(...answerCells.map(c => c.id))
  else {
   let partial = 0, step = 0, started = false
   for (let index = 0; index < leftDigits; index++) {
    partial = partial * 10 + Number(String(left)[index])
    if (!started && partial < right && index < leftDigits - 1) continue
    const row = 1 + step * 3, end = index + 1
    if (started) {
     addField(`bring_${step}`, 1, row, end)
     const prior = session.fields.find(f => f.key === `bring_${step}`)
     if (!prior) addNumber(String(left)[index], row, end)
    }
    if (answerCells[step]) order.push(answerCells[step].id)
    const product = Math.floor(partial / right) * right
    addField(`product_${step}`, String(product).length, row + 1, end, true)
    marks.push({ text: '−', row: row + 1, col: 0 }, { text: '', row: row + 1, col: Math.max(1, end - String(partial).length + 1), width: String(partial).length, line: true })
    partial -= product
    addField(`remainder_${step}`, String(partial).length, row + 2, end, true)
    // The next partial reuses the child's remainder beside the brought-down digit.
    if (index < leftDigits - 1) marks.push({ text: `remainder_${step}`, row: row + 3, col: end - String(partial).length + 1, width: String(partial).length })
    started = true; step++; rows = row + 2
   }
  }
 }
 return { cells, marks, order, widths, rows, size, division, multiplication, arithmetic }
}

