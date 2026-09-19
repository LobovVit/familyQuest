import { useRef, useState, type FormEvent } from 'react'
import type { Participant } from '../../domain/models'
import { emptyDraft, kindLabels, weekdays, type FamilyDraft, type FamilyEntry, type FamilyKind } from '../../domain/family'
import { presets } from './library'
import { readMedia } from './media'

type Props = { kind: FamilyKind; initial?: FamilyDraft; entry?: FamilyEntry; participants: Participant[]; busy: boolean; onSave: (draft: FamilyDraft) => Promise<boolean>; onClose: () => void }
export function FamilyEditor({ kind, initial, entry, participants, busy, onSave, onClose }: Props) {
  const [draft, setDraft] = useState<FamilyDraft>(() => initial ?? emptyDraft())
  const [mediaBusy, setMediaBusy] = useState(false)
  const [error, setError] = useState('')
  const mediaLock = useRef(false)
  const stepLocked = !!entry?.events.length
  const isAdventure = kind === 'adventure' || kind === 'proposal'
  function patch(fields: Partial<FamilyDraft>) { setDraft(d => ({ ...d, ...fields })) }
  async function submit(e: FormEvent) {
    e.preventDefault()
    if (busy || mediaBusy) return
    if (isAdventure && draft.steps.length === 0) { setError('Добавьте хотя бы один шаг'); return }
    if (kind === 'habit' && draft.weekdays.length === 0) { setError('Выберите дни недели'); return }
    setError('')
    if (await onSave(draft)) onClose()
  }
  async function upload(file: File | undefined, kind: 'photo' | 'audio') {
    if (!file || mediaLock.current) return
    mediaLock.current = true; setMediaBusy(true); setError('')
    try { patch({ [kind]: await readMedia(file, kind) }) }
    catch (e) { setError(e instanceof Error ? e.message : 'Не удалось загрузить файл') }
    finally { mediaLock.current = false; setMediaBusy(false) }
  }
  return <section className="family-editor" aria-label="Редактор семейной карточки">
    <div className="family-row"><h2>{entry ? 'Редактировать' : 'Новая карточка'} · {kindLabels[kind]}</h2><button type="button" disabled={busy || mediaBusy} onClick={onClose}>Закрыть</button></div>
    {!entry && presets[kind] && <div className="family-presets" aria-label="Готовые варианты">{presets[kind]?.map(p => <button key={p.title} type="button" onClick={() => patch({ ...p, easyVersion: p.easyVersion ?? '', value: p.value ?? '' })}>{p.title}</button>)}</div>}
    <form onSubmit={submit}>
      <fieldset disabled={busy || mediaBusy}>
        <label>Название<input required maxLength={160} value={draft.title} onChange={e => patch({ title: e.target.value })} /></label>
        <label>{kind === 'memory' ? 'История, смешная фраза или впечатление' : kind === 'value' ? 'Как проявим ценность в поступке' : 'Описание и подготовка'}<textarea required={kind === 'value'} rows={4} maxLength={4000} value={draft.description} onChange={e => patch({ description: e.target.value })} /></label>
        <div className="family-fields">
          <label>{kind === 'habit' ? 'Начинаем с' : 'Дата'}<input type="date" onInput={e => patch({ date: e.currentTarget.value })} required min="2000-01-01" max="2100-12-31" value={draft.date} onChange={e => patch({ date: e.target.value })} /></label>
          <label>Ценность семьи<input required={kind === 'value'} maxLength={100} placeholder="Например, забота" value={draft.value} onChange={e => patch({ value: e.target.value })} /></label>
        </div>
        <fieldset className="family-people"><legend>Кто участвует · без выбора — вся семья</legend>{participants.map(p => <label key={p.id}><input type="checkbox" checked={draft.participantIds.includes(p.id)} onChange={() => patch({ participantIds: draft.participantIds.includes(p.id) ? draft.participantIds.filter(id => id !== p.id) : [...draft.participantIds, p.id] })} />{p.name}</label>)}</fieldset>
        {kind === 'habit' && <>
          <fieldset className="family-people"><legend>Дни недели</legend>{weekdays.map(d => <label key={d.id}><input type="checkbox" checked={draft.weekdays.includes(d.id)} onChange={() => patch({ weekdays: draft.weekdays.includes(d.id) ? draft.weekdays.filter(v => v !== d.id) : [...draft.weekdays, d.id] })} />{d.label}</label>)}</fieldset>
          <label>Напомнить в календаре<input type="time" onInput={e => patch({ reminder: e.currentTarget.value })} value={draft.reminder} onChange={e => patch({ reminder: e.target.value })} /><small>После сохранения можно скачать напоминание в календарь. В приложении привычка появится в плане дня.</small></label>
          <label>Облегчённый вариант на сложный день<textarea maxLength={4000} value={draft.easyVersion} onChange={e => patch({ easyVersion: e.target.value })} /></label>
        </>}
        {isAdventure && <fieldset><legend>План и роли</legend>
          {stepLocked && <p className="family-muted">Шаги с историей участия сохраняются. Для другого плана создайте новое приключение.</p>}
          {draft.steps.map((step, index) => <div className="family-step-edit" key={index}>
            <label>Шаг {index + 1}<input required maxLength={500} disabled={stepLocked} value={step.title} onChange={e => patch({ steps: draft.steps.map((s, i) => i === index ? { ...s, title: e.target.value } : s) })} /></label>
            <label>Ответственный<select disabled={stepLocked} value={step.participantId} onChange={e => patch({ steps: draft.steps.map((s, i) => i === index ? { ...s, participantId: Number(e.target.value) } : s) })}><option value={0}>Вместе</option>{participants.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
            {!stepLocked && <button type="button" aria-label={`Удалить шаг ${index + 1}`} onClick={() => patch({ steps: draft.steps.filter((_, i) => i !== index) })}>×</button>}
          </div>)}
          {!stepLocked && draft.steps.length < 30 && <button type="button" onClick={() => patch({ steps: [...draft.steps, { title: '', participantId: 0 }] })}>+ Добавить шаг</button>}
        </fieldset>}
        {kind === 'skill' && <p className="family-muted">У каждого участника свой путь: посмотрел → сделал вместе → попробовал сам → объяснил другому.</p>}
        {kind === 'proposal' && <p className="family-muted">Семья увидит идею сразу. Родитель согласует план, дату и условия перед началом.</p>}
        {kind === 'council' && <>
          <label>Одна договорённость<textarea maxLength={4000} placeholder="Что попробуем изменить и кто поможет?" value={draft.agreement} onChange={e => patch({ agreement: e.target.value })} /></label>
          <label>Следующее совместное занятие<textarea maxLength={4000} value={draft.nextActivity} onChange={e => patch({ nextActivity: e.target.value })} /></label>
        </>}
        {kind === 'memory' && <div className="family-fields">
          <label>Фото · по желанию<input type="file" accept="image/jpeg,image/png" onChange={e => { void upload(e.target.files?.[0], 'photo'); e.target.value = '' }} /><small>JPEG/PNG до 15 МБ, фото будет уменьшено.</small>{draft.photo && <><img className="family-photo" src={draft.photo} alt="Фото воспоминания" /><button type="button" onClick={() => patch({ photo: '' })}>Убрать фото</button></>}</label>
          <label>Голосовая история · по желанию<input type="file" accept="audio/mpeg,audio/wav,audio/ogg,audio/webm,audio/mp4,.m4a" onChange={e => { void upload(e.target.files?.[0], 'audio'); e.target.value = '' }} /><small>Короткая запись до 500 КБ.</small>{draft.audio && <><audio controls src={draft.audio} /><button type="button" onClick={() => patch({ audio: '' })}>Убрать запись</button></>}</label>
        </div>}
        {error && <p role="alert" className="notice">{error}</p>}
        <div className="family-row"><button type="submit">{busy ? 'Сохраняем…' : 'Сохранить'}</button><button type="button" onClick={onClose}>Отмена</button></div>
      </fieldset>
      {mediaBusy && <p role="status">Подготавливаем файл…</p>}
    </form>
  </section>
}
