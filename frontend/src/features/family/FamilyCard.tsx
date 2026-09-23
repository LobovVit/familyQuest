import { useState } from 'react'
import type { Participant } from '../../domain/models'
import { kindIcons, kindLabels, localDate, skillStage, stageLabels, weekdays, type FamilyAction, type FamilyEntry } from '../../domain/family'
import { downloadCalendar } from './calendar'

type Props = { entry: FamilyEntry; current: Participant; participants: Participant[]; date: string; busy: boolean; onEdit: () => void; onRepeat: () => void; onMemory: () => void; onPlan: () => void; onAction: (action: FamilyAction) => Promise<boolean> }
export function FamilyCard({ entry: e, current, participants, date, busy, onEdit, onRepeat, onMemory, onPlan, onAction }: Props) {
  const [note, setNote] = useState('')
  const [showHistory, setShowHistory] = useState(false)
  const name = (id: number) => participants.find(p => p.id === id)?.name ?? 'Участник семьи'
  const canEdit = current.role === 'parent' || e.authorId === current.id
  const involved = !(e.participantIds ?? []).length || e.participantIds.includes(current.id)
  const canAct = !e.archived && (e.kind !== 'proposal' || e.approved) && date <= localDate()
  const checked = e.events.some(v => v.kind === 'checkin' && v.actorId === current.id && v.date === date)
  const stage = skillStage(e, current.id)
  async function act(action: string, step?: number, easy = false) {
    if (await onAction({ action, date: action === 'repeat' ? localDate() : date, note, step, easy })) setNote('')
  }
  return <article className={`family-card ${e.archived ? 'is-archived' : ''}`}>
    <div className="family-row"><span className="family-tag">{kindIcons[e.kind]} {kindLabels[e.kind]}</span><small>{e.date && new Date(`${e.date}T12:00:00`).toLocaleDateString('ru-RU')}</small></div>
    <h3>{e.title}</h3>
    {e.kind === 'habit' && <p className="family-tag">Награда за участие по расписанию: 5 ⭐ + 1 🙂</p>}
    {['adventure', 'proposal'].includes(e.kind) && <p className="family-tag">После всех шагов: 40 ⭐ + 3 🙂 каждому участнику</p>}
    {e.value && <p className="family-value">{e.value}</p>}
    {e.description && <p className="family-text">{e.description}</p>}
    <p className="family-muted">{(e.participantIds ?? []).length ? e.participantIds.map(name).join(', ') : 'Вся семья'} · Автор: {name(e.authorId)}</p>
    {e.archived && <p className="family-tag">В архиве · история сохранена</p>}
    {e.kind === 'proposal' && <div className="family-callout">{e.approved ? 'План согласован — можно начинать' : 'Идея ждёт согласования плана, даты и условий'}{!e.approved && !e.archived && current.role === 'parent' && <button disabled={busy} onClick={() => void onAction({ action: 'approve' })}>Согласовать план</button>}</div>}
    {(e.kind === 'adventure' || e.kind === 'proposal') && <ol className="family-steps">{(e.steps ?? []).map((step, i) => {
      const done = e.events.find(v => v.kind === 'step' && v.step === i)
      return <li key={i}><div><strong>{step.title}</strong><small>{done ? `✓ ${name(done.actorId)} · ${done.date}` : step.participantId ? name(step.participantId) : 'Вместе'}</small></div>{!done && canAct && involved && (!step.participantId || step.participantId === current.id) && <button disabled={busy} onClick={() => void act('step', i)}>Готово</button>}</li>
    })}</ol>}
    {e.kind === 'habit' && <><p>{weekdays.filter(d => (e.weekdays ?? []).includes(d.id)).map(d => d.label).join(' · ')}{e.reminder && ` · ${e.reminder}`}</p>{e.easyVersion && <p className="family-callout">На сложный день: {e.easyVersion}</p>}<p className="family-muted">Дней участия: {new Set(e.events.filter(v => v.kind === 'checkin').map(v => v.date)).size}. После перерыва можно просто продолжить.</p>{e.reminder && <button onClick={() => downloadCalendar(e)}>Скачать напоминание в календарь</button>}</>}
    {['habit', 'value'].includes(e.kind) && canAct && involved && <div className="family-row"><button disabled={busy || checked} onClick={() => void act('checkin')}>{checked ? '✓ Мой вклад отмечен' : 'Я участвовал(а)'}</button>{e.kind === 'habit' && e.easyVersion && !checked && <button disabled={busy} onClick={() => void act('checkin', 0, true)}>Сделали облегчённый вариант</button>}</div>}
    {e.kind === 'skill' && <><ol className="family-skill-stages">{stageLabels.map((label, i) => <li key={label} className={i < stage ? 'is-done' : ''}>{i < stage ? '✓ ' : `${i + 1}. `}{label}</li>)}</ol>{canAct && involved && stage < 4 && <button disabled={busy} onClick={() => void act('stage', stage + 1)}>Мой этап: {stageLabels[stage]}</button>}<details><summary>Прогресс участников</summary>{participants.filter(p => !(e.participantIds ?? []).length || e.participantIds.includes(p.id)).map(p => <p key={p.id}>{p.name}: {skillStage(e, p.id)} из 4 этапов</p>)}</details></>}
    {e.kind === 'council' && <><div className="family-callout"><strong>Договорились</strong><p className="family-text">{e.agreement || 'После обсуждения сохраните одну общую договорённость.'}</p></div>{e.nextActivity && <div className="family-callout"><strong>Попробуем вместе</strong><p>{e.nextActivity}</p><button disabled={busy} onClick={onPlan}>Запланировать занятие</button></div>}<p className="family-muted">Можно ответить на любой вопрос или пропустить. Заметки видны всей семье.</p></>}
    {e.kind === 'memory' && <>{e.photo && <img loading="lazy" className="family-photo" src={e.photo} alt={e.title} />}{e.audio && <audio controls preload="none" src={e.audio} />}</>}
    {canAct && <div className="family-note"><label>{e.kind === 'council' ? 'Моё предложение или благодарность' : 'Что получилось? Что было трудно?'}<textarea rows={2} maxLength={2000} value={note} onChange={event => setNote(event.target.value)} placeholder="По желанию · видно всей семье" /></label><button disabled={busy || !note.trim()} onClick={() => void act('reflection')}>Добавить заметку</button></div>}
    {!e.archived && <div className="family-row"><button disabled={busy || e.events.some(v => v.kind === 'repeat' && v.actorId === current.id)} onClick={() => void act('repeat')}>♡ {e.events.some(v => v.kind === 'repeat' && v.actorId === current.id) ? 'Хочу повторить' : 'Мне хочется повторить'}</button>{['adventure', 'proposal', 'memory', 'council'].includes(e.kind) && <button disabled={busy} onClick={onRepeat}>Создать продолжение</button>}{e.kind !== 'memory' && <button disabled={busy} onClick={onMemory}>Сохранить воспоминание</button>}</div>}
    <div className="family-row">{canEdit && <>{!e.archived && <button disabled={busy} onClick={onEdit}>Изменить</button>}<button disabled={busy} onClick={() => void onAction({ action: e.archived ? 'restore' : 'archive' })}>{e.archived ? 'Вернуть из архива' : 'В архив'}</button></>}<button aria-expanded={showHistory} onClick={() => setShowHistory(v => !v)}>История ({e.events.length})</button></div>
    {showHistory && <ul className="family-history">{e.events.length === 0 && <li>История появится после первого участия.</li>}{e.events.slice().reverse().map((v, i) => <li key={i}><strong>{name(v.actorId)}</strong> · {v.date} · {({ checkin: 'Участвовал(а)', step: `Шаг ${v.step + 1}`, stage: stageLabels[v.step - 1], reflection: 'Заметка', repeat: 'Хочет повторить' } as Record<string, string>)[v.kind]}{v.easy && ' · облегчённый вариант'}{v.note && <p className="family-text">{v.note}</p>}</li>)}</ul>}
  </article>
}
