import type { Dispatch, SetStateAction } from 'react'
import type { BenefitType, ChoreDraft, Participant, Schedule, TimeWindow } from '../../domain/models'
import { participantAvatar } from '../../domain/avatar'
import { benefitLabels, scheduleLabels, windowLabels } from '../shared/labels'
export function ChoreEditor({
  draft,
  onCancel,
  onSave,
  onToggleParticipant,
  participants,
  setDraft,
}: {
  draft: ChoreDraft
  onCancel: () => void
  onSave: () => void
  onToggleParticipant: (participantId: number) => void
  participants: Participant[]
  setDraft: Dispatch<SetStateAction<ChoreDraft>>
}) {
  return (
    <div className="chore-editor">
      <label>
        Название
        <input value={draft.title} onChange={(event) => setDraft({ ...draft, title: event.target.value })} />
      </label>
      <label>
        Описание
        <textarea value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
      </label>
      <div className="form-row">
        <label>
          Периодичность
          <select
            value={draft.schedule}
            onChange={(event) => {
              const schedule = event.target.value as Schedule
              setDraft({ ...draft, schedule, timeWindow: schedule === 'daily' ? draft.timeWindow : '' })
            }}
          >
            {Object.entries(scheduleLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Когда
          <select
            disabled={draft.schedule !== 'daily'}
            value={draft.schedule === 'daily' ? draft.timeWindow : ''}
            onChange={(event) => setDraft({ ...draft, timeWindow: event.target.value as TimeWindow })}
          >
            {Object.entries(windowLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label>
        Польза
        <select value={draft.benefitType} onChange={(event) => setDraft({ ...draft, benefitType: event.target.value as BenefitType })}>
          {Object.entries(benefitLabels).map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Базовая ценность
        <input min="1" type="number" value={draft.baseValue} onChange={(event) => setDraft({ ...draft, baseValue: Number(event.target.value) })} />
      </label>
      <div className="participant-picker" aria-label="Участники обязанности">
        {participants.map((person) => (
          <label className={draft.participantIds.includes(person.id) ? 'active' : ''} key={person.id}>
            <input checked={draft.participantIds.includes(person.id)} onChange={() => onToggleParticipant(person.id)} type="checkbox" />
            <span>{participantAvatar(person)}</span>
            <strong>{person.name}</strong>
          </label>
        ))}
      </div>
      <div className="editor-actions">
        <button type="button" onClick={onCancel}>
          Отмена
        </button>
        <button type="button" onClick={onSave}>
          Сохранить
        </button>
      </div>
    </div>
  )
}
