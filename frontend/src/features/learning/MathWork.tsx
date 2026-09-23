import { useState } from 'react'
import { mathSymbols, type MathField, type MathView } from '../../domain/learning'
export function MathWork({ session, busy, onSubmit }: { session: MathView; busy: boolean; onSubmit: (values: Record<string, number>) => Promise<void> }) {
 const q = session.question!, column = session.settings.level === 'columnar', division = session.settings.operation === ':'
 const [values, setValues] = useState<Record<string, string>>({}), [selected, setSelected] = useState('answer'), [error, setError] = useState('')
 const maxDigits = (f: MathField) => column ? f.width : 7
 const fields = session.fields
 const answer = fields.find(f => f.key === 'answer')!
 const isCarry = (key: string) => key.startsWith('carry_') || key.startsWith('mulcarry_')
 function update(key: string, value: string) {
  const negative = !column && session.settings.operation === '-' && session.settings.allowNegative && value.startsWith('-')
  const f = fields.find(x => x.key === key)!
  setValues(old => ({ ...old, [key]: (negative ? '-' : '') + value.replace(/\D/g, '').slice(0, maxDigits(f)) }))
 }
 function field(f: MathField, compact = false) {
  return <label key={f.key} className={`math-field ${compact ? 'math-carry' : ''}`}><span>{compact ? `↑${Number(f.key.split('_').at(-1)) + 1}` : f.label}</span><input aria-label={f.label} className={selected === f.key ? 'is-selected' : ''} inputMode="numeric" autoComplete="off" value={values[f.key] ?? ''} placeholder={isCarry(f.key) ? '0' : ''} maxLength={maxDigits(f) + 1} style={column ? { width: `${Math.max(f.width, 1) * 2.4 + 1}rem`, marginRight: `${f.shift * 2.4}rem` } : undefined} onFocus={() => setSelected(f.key)} onChange={e => update(f.key, e.target.value)} /></label>
 }
 function press(digit: string) { update(selected, (values[selected] ?? '') + digit) }
 async function submit(choice?: number) {
  if (busy) return
  const payload: Record<string, number> = {}
  for (const f of fields) {
   const text = f.key === 'answer' && choice !== undefined ? String(choice) : values[f.key] ?? ''
   if (!/^-?\d+$/.test(text) && !(isCarry(f.key) && !text)) { setError('Заполни ответ и все шаги. Если переноса или займа нет, оставь 0.'); return }
   payload[f.key] = Number(text || '0')
  }
  setError(''); await onSubmit(payload)
 }
 return <form className="math-work" onSubmit={e => { e.preventDefault(); void submit() }}><fieldset disabled={busy}>
 <h2>Пример {session.index + 1} из {session.total}</h2>
 {column ? <><p className="family-muted">{division ? 'Дели слева направо. После вычитания сноси следующую цифру.' : 'Начинай с единиц справа. Записывай переносы и промежуточные результаты.'}</p><div className="math-paper">
 {division ? <><div className="math-division-head"><strong>{q.left}</strong><div><strong>{q.right}</strong>{field(answer)}</div></div><div className="math-division-steps">{fields.filter(f => f.key !== 'answer').map(f => field(f))}</div></> : <>
 {['+', '-'].includes(session.settings.operation) && <div className="math-carries">{fields.filter(f => isCarry(f.key)).reverse().map(f => field(f, true))}</div>}
 <div className="math-operands" aria-label={`${q.left} ${mathSymbols[session.settings.operation]} ${q.right}`}><div>{q.left}</div><div><span>{mathSymbols[session.settings.operation]}</span>{q.right}</div></div>
 {session.settings.operation === '*' && <div className="math-partials">{fields.filter(f => f.key.startsWith('partial_')).map(f => <div className="math-partial-row" key={f.key}><div className="math-carries">{fields.filter(c => c.key.startsWith(`mulcarry_${f.key.split('_')[1]}_`)).reverse().map(c => field(c, true))}</div>{field(f)}</div>)}</div>}
 <div className="math-result">{field(answer)}</div>
 </>}
 </div></> : <div className="math-inline" aria-label={`${q.left} ${mathSymbols[session.settings.operation]} ${q.right}`}><strong>{q.left} {mathSymbols[session.settings.operation]} {q.right} = ?</strong></div>}
 {session.settings.answerMode === 'choice' ? <div className="math-choices">{q.options.map(v => <button type="button" key={v} onClick={() => void submit(v)}>{v}</button>)}</div> : <>
 {!column && field(answer)}
 <div className="math-keypad-area"><p>Выбрано: {fields.find(f => f.key === selected)?.label}</p><div className="math-keypad" aria-label="Цифровая клавиатура">{['1','2','3','4','5','6','7','8','9'].map(n => <button type="button" key={n} onClick={() => press(n)}>{n}</button>)}<button type="button" onClick={() => update(selected, '')}>Очистить</button><button type="button" onClick={() => press('0')}>0</button><button type="button" aria-label="Удалить цифру" onClick={() => update(selected, (values[selected] ?? '').slice(0, -1))}>⌫</button>{!column && session.settings.operation === '-' && session.settings.allowNegative && <button type="button" onClick={() => update(selected, (values[selected] ?? '').startsWith('-') ? (values[selected] ?? '').slice(1) : `-${values[selected] ?? ''}`)}>±</button>}</div></div>
 {error && <p role="alert">{error}</p>}<button type="submit" className="family-primary math-check">{busy ? 'Проверяем…' : 'Проверить'}</button>
 </>}
 </fieldset></form>
}
