import { useRef, useState, type CSSProperties } from 'react'
import type { MathView } from '../../domain/learning'

import { columnLayout, type ColumnCell } from './columnLayout'

export function ColumnWork({ session, busy, onSubmit }: { session: MathView; busy: boolean; onSubmit: (values: Record<string, number>) => Promise<void> }) {
 const layout = columnLayout(session)
 const [values, setValues] = useState<Record<string, string>>({})
 const [selected, setSelected] = useState(layout.order[0]), [error, setError] = useState('')
 const inputs = useRef<Record<string, HTMLInputElement | null>>({})
 const active = layout.cells.find(c => c.id === selected)
 const inputCells = [...layout.order.map(id => layout.cells.find(c => c.id === id)!), ...layout.cells.filter(c => c.memo)]
 function enabled(c: ColumnCell) {
  if (!layout.arithmetic || c.carry || c.id === selected || c.col === layout.size) return true
  return layout.cells.some(other => other.key === 'answer' && other.col > c.col && !!values[other.id])
 }
 function dotEnabled(c: ColumnCell) { return !!active && (c.col === active.col - 1 || (active.col === 2 && c.col === 2)) }
 const partialIndex = active?.key.startsWith('partial_') ? Number(active.key.split('_')[1]) : active?.key.startsWith('mulcarry_') ? Number(active.key.split('_')[1]) : undefined
 const topColumn = active ? active.col + (active.carry ? 1 : 0) + (partialIndex ?? 0) : undefined
 const bottomColumn = layout.multiplication ? partialIndex === undefined ? undefined : layout.size - partialIndex : topColumn
 const instruction = layout.division ? 'Дели слева направо: цифра частного → умножение → вычитание → снос следующей цифры. Не пропускай нули в частном.' : layout.arithmetic ? 'Выбирай клетки ответа справа налево и вписывай по одной цифре. Нажми на точку над первым числом, чтобы отметить или убрать перенос в следующий разряд.' : 'Выбирай клетки справа налево. Ввод цифры не меняет выбранную клетку. Над каждой строкой записывай цифры «в уме». Строки сдвигаются влево. При сложении строк тоже можно записать число «в уме».'
 function write(id: string, value: string) {
  const cell = layout.cells.find(c => c.id === id)
  if (busy || !cell || !enabled(cell) || !/^\d?$/.test(value)) return
  const digit = value
  setValues(old => ({ ...old, [id]: digit }))
  if (layout.division && digit && layout.order.includes(id)) {
   const next = layout.order[layout.order.indexOf(id) + 1]
   if (next) { setSelected(next); inputs.current[next]?.focus() }
  }
 }
 function read(key: string) { return Array.from({ length: layout.widths[key] ?? 0 }, (_, i) => values[`${key}:${i}`] ?? '').join('') }
 async function submit() {
  if (busy) return
  const missing = inputCells.find(c => !c.carry && !values[c.id])
  if (missing) { setError('Заполни все клетки решения. Если переноса нет, его клетку можно оставить пустой.'); setSelected(missing.id); inputs.current[missing.id]?.focus(); return }
  const payload = Object.fromEntries(session.fields.map(f => [f.key, Number(read(f.key) || '0')]))
  // RU: Старший разряд уже записан ребёнком в произведении, отдельный перенос не нужен.
  // EN: The child already entered the leading product digit; no duplicate carry cell is needed.
  if (layout.multiplication) {
   const digits = String(session.question!.left).length
   for (let i = 0; i < String(session.question!.right).length; i++) {
    const key = `mulcarry_${i}_${digits - 1}`
    if (key in payload) payload[key] = Math.floor(payload[`partial_${i}`] / 10 ** digits)
   }
  }
  if (layout.multiplication && String(session.question!.right).length === 1) payload.answer = payload.partial_0
  setError(''); await onSubmit(payload)
 }
 return <form className="math-work" onSubmit={e => { e.preventDefault(); void submit() }}><fieldset disabled={busy}>
  <h2>Пример {session.index + 1} из {session.total}</h2>
  <p>{instruction}</p>
  <div className="math-paper"><div className={`column-notebook ${layout.arithmetic ? 'column-arithmetic' : layout.multiplication ? 'column-multiplication' : ''}`} style={{ '--columns': layout.division ? layout.size + String(session.question!.left / session.question!.right).length + 3 : layout.size + 1, '--rows': layout.rows } as CSSProperties}>
   {layout.division && <div className="column-bracket" style={{ gridColumn: layout.size + 2, gridRow: '1 / 3' }} />}
   {layout.marks.map((m, i) => <span key={i} className={`column-mark ${m.line ? 'column-line' : ''} ${m.kind ? `column-${m.kind}` : ''} ${m.operand && m.col === (m.operand === 'top' ? topColumn : bottomColumn) ? 'column-active-operand' : ''}`} style={{ gridColumn: `${m.col + 1} / span ${m.width ?? 1}`, gridRow: m.row }}>{m.text.startsWith('remainder_') ? (Number(read(m.text)) === 0 ? '' : [...read(m.text)].map((digit, i) => <span key={i} style={{ width: 'var(--cell)', textAlign: 'center' }}>{digit}</span>)) : m.text}</span>)}
   {layout.cells.filter(c => c.dot).map(c => <span key={c.id} className="column-dot-anchor" style={{ gridColumn: c.col + 1, gridRow: c.row }}><button type="button" className="column-dot" disabled={!dotEnabled(c)} aria-label={c.label} aria-pressed={values[c.id] === '1'} title={`${c.label}: в уме`} onClick={() => setValues(old => ({ ...old, [c.id]: old[c.id] === '1' ? '' : '1' }))}><span aria-hidden="true" /></button></span>)}
   {inputCells.map(c => <input key={c.id} ref={el => { inputs.current[c.id] = el }} aria-label={c.label} title={c.label} disabled={!enabled(c)} inputMode="numeric" autoComplete="off" maxLength={1} className={`column-digit ${c.carry ? 'column-carry' : ''} ${selected === c.id ? 'is-selected' : ''}`} style={{ gridColumn: c.col + 1, gridRow: c.row }} value={values[c.id] ?? ''} placeholder="" onFocus={e => { setSelected(c.id); e.target.select() }} onChange={e => write(c.id, e.target.value)} onPaste={e => { e.preventDefault(); write(c.id, e.clipboardData.getData('text')) }} onKeyDown={e => { if (/^[0-9]$/.test(e.key)) { e.preventDefault(); write(c.id, e.key) } }} />)}
  </div></div>
  <div className="math-keypad-area"><p aria-live="polite">{active?.label}</p><div className="math-keypad" aria-label="Цифровая клавиатура">{['1','2','3','4','5','6','7','8','9'].map(n => <button type="button" key={n} onClick={() => write(selected, n)}>{n}</button>)}<button type="button" onClick={() => { setValues({}); setSelected(layout.order[0]) }}>Очистить</button><button type="button" onClick={() => write(selected, '0')}>0</button><button type="button" aria-label="Удалить цифру" onClick={() => write(selected, '')}>⌫</button></div></div>
  {error && <p role="alert">{error}</p>}<button type="submit" className="family-primary">{busy ? 'Проверяем…' : 'Проверить'}</button>
 </fieldset></form>
}
