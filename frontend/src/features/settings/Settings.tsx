import type { ChangeEvent, Dispatch, FormEvent, SetStateAction } from 'react'
import type { Participant, Reward, RewardPeriod, RewardType, Task } from '../../domain/models'
import { avatarForRole, participantAvatar } from '../../domain/avatar'
import { rewardPeriodLabels, rewardTypeLabels, roleLabels } from '../shared/labels'

export type ParticipantDraft={name:string;role:Participant['role'];pin:string}
export type RewardDraft={title:string;description:string;period:RewardPeriod;rewardType:RewardType;starCost:number;smileCost:number;participantIds:number[]}
type Props={participants:Participant[];tasks:Task[];rewards:Reward[];pinEdit:{participantId:number;pin:string}|null;setPinEdit:Dispatch<SetStateAction<{participantId:number;pin:string}|null>>;newParticipant:ParticipantDraft;setNewParticipant:Dispatch<SetStateAction<ParticipantDraft>>;newReward:RewardDraft;setNewReward:Dispatch<SetStateAction<RewardDraft>>;backupBusy:boolean;onSavePin:(p:Participant)=>void;onDeleteParticipant:(p:Participant)=>void;onCreateParticipant:(e:FormEvent)=>void;onExport:()=>void;onImport:(e:ChangeEvent<HTMLInputElement>)=>void;onDeleteReward:(r:Reward)=>void;onCreateReward:(e:FormEvent)=>void;onToggleRewardParticipant:(id:number)=>void}
export function Settings(props:Props){const {participants,tasks,rewards,pinEdit,setPinEdit,newParticipant,setNewParticipant,newReward,setNewReward,backupBusy:isBackupBusy,onSavePin:saveParticipantPIN,onDeleteParticipant:deleteParticipant,onCreateParticipant:createParticipant,onExport:exportBackup,onImport:importBackup,onDeleteReward:deleteReward,onCreateReward:createReward,onToggleRewardParticipant:toggleRewardParticipant}=props;return (
        <section className="users-grid">
          {participants.map((person) => (
            <article className="user-card" key={person.id}>
              <span className={`avatar ${person.role}`}>{participantAvatar(person)}</span>
              <div>
                <h2>{person.name}</h2>
                <p>{roleLabels[person.role]}</p>
              </div>
              <dl>
                <div>
                  <dt>PIN</dt>
                  <dd>6 цифр, задан</dd>
                </div>
                <div>
                  <dt>Сегодня</dt>
                  <dd>{tasks.filter((task) => task.participantId === person.id).length} дел</dd>
                </div>
              </dl>
              {pinEdit?.participantId === person.id ? (
                <div className="pin-edit">
                  <input
                    autoFocus
                    inputMode="numeric"
                    maxLength={6}
                    value={pinEdit.pin}
                    onChange={(event) =>
                      setPinEdit({
                        participantId: person.id,
                        pin: event.target.value.replace(/\D/g, '').slice(0, 6),
                      })
                    }
                  />
                  <button disabled={pinEdit.pin.length !== 6} type="button" onClick={() => saveParticipantPIN(person)}>
                    Сохранить
                  </button>
                  <button type="button" onClick={() => setPinEdit(null)}>
                    Отмена
                  </button>
                </div>
              ) : (
                <div className="user-actions">
                  <button type="button" onClick={() => setPinEdit({ participantId: person.id, pin: '' })}>
                    Изменить PIN
                  </button>
                  <button type="button" onClick={() => deleteParticipant(person)}>
                    Удалить
                  </button>
                </div>
              )}
            </article>
          ))}
          <section className="panel">
            <h2>Добавить пользователя</h2>
            <form className="stack-form" onSubmit={createParticipant}>
              <label>
                Имя
                <input value={newParticipant.name} onChange={(event) => setNewParticipant({ ...newParticipant, name: event.target.value })} />
              </label>
              <div className="form-row">
                <label>
                  Роль
                  <select
                    value={newParticipant.role}
                    onChange={(event) => setNewParticipant({ ...newParticipant, role: event.target.value as Participant['role'] })}
                  >
                    <option value="child">Дошкольник</option>
                    <option value="school">Школьник</option>
                    <option value="parent">Взрослый</option>
                  </select>
                </label>
                <label>
                  PIN
                  <input
                    inputMode="numeric"
                    maxLength={6}
                    value={newParticipant.pin}
                    onChange={(event) => setNewParticipant({ ...newParticipant, pin: event.target.value.replace(/\D/g, '').slice(0, 6) })}
                  />
                </label>
              </div>
              <button type="submit">Добавить пользователя</button>
            </form>
          </section>

          <section className="panel backup-panel">
            <div>
              <h2>Данные системы</h2>
              <p>Полная выгрузка пользователей, обязанностей, задач, рейтингов, наград, PIN-кодов и истории выполнения.</p>
            </div>
            <div className="backup-actions">
              <button disabled={isBackupBusy} type="button" onClick={exportBackup}>
                Скачать файл
              </button>
              <label className={isBackupBusy ? 'disabled' : ''}>
                Загрузить файл
                <input accept="application/json,.json" disabled={isBackupBusy} type="file" onChange={importBackup} />
              </label>
            </div>
            <p className="settings-note">Для первичного запуска положите этот файл как backend/seed/familyquest-backup.json перед docker-compose up.</p>
          </section>

          <section className="panel rewards-panel">
            <div className="section-heading compact">
              <h2>Награды</h2>
            </div>
            <div className="reward-list">
              {rewards.length === 0 && <p className="settings-note">Добавьте первые награды для обмена звездочек или чемпионства.</p>}
              {rewards.map((reward) => (
                <article className="reward-card" key={reward.id}>
                  <div>
                    <h3>{reward.title}</h3>
                    <p>{reward.description}</p>
                  </div>
                  <div className="tag-row">
                    <span>{rewardTypeLabels[reward.rewardType]}</span>
                    <span>{rewardPeriodLabels[reward.period]}</span>
                    {reward.rewardType === 'stars' && <span>{reward.starCost} ⭐</span>}
                    {reward.rewardType === 'smiles' && <span>{reward.smileCost} 🙂</span>}
                  </div>
                  <div className="assignee-row">
                    {reward.participantNames.map((name) => (
                      <span key={name}>{avatarForRole()} {name}</span>
                    ))}
                  </div>
                  <button type="button" onClick={() => deleteReward(reward)}>
                    Удалить
                  </button>
                </article>
              ))}
            </div>
          </section>

          <section className="panel rewards-panel">
            <h2>Добавить награду</h2>
            <form className="stack-form" onSubmit={createReward}>
              <label>
                Название
                <input value={newReward.title} onChange={(event) => setNewReward({ ...newReward, title: event.target.value })} />
              </label>
              <label>
                Описание
                <textarea value={newReward.description} onChange={(event) => setNewReward({ ...newReward, description: event.target.value })} />
              </label>
              <div className="form-row">
                <label>
                  Период
                  <select value={newReward.period} onChange={(event) => setNewReward({ ...newReward, period: event.target.value as RewardPeriod })}>
                    {Object.entries(rewardPeriodLabels).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Тип
                  <select
                    value={newReward.rewardType}
                    onChange={(event) => setNewReward({ ...newReward, rewardType: event.target.value as RewardType })}
                  >
                    {Object.entries(rewardTypeLabels).map(([value, label]) => (
                      <option key={value} value={value}>
                        {label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              {newReward.rewardType === 'stars' && (
                <label>
                  Стоимость в звездочках
                  <input
                    min="1"
                    type="number"
                    value={newReward.starCost}
                    onChange={(event) => setNewReward({ ...newReward, starCost: Number(event.target.value) })}
                  />
                </label>
              )}
              {newReward.rewardType === 'smiles' && (
                <label>
                  Стоимость в улыбках
                  <input
                    min="1"
                    type="number"
                    value={newReward.smileCost}
                    onChange={(event) => setNewReward({ ...newReward, smileCost: Number(event.target.value) })}
                  />
                </label>
              )}
              <div className="participant-picker" aria-label="Пользователи награды">
                {participants.map((person) => (
                  <label className={newReward.participantIds.includes(person.id) ? 'active' : ''} key={person.id}>
                    <input checked={newReward.participantIds.includes(person.id)} onChange={() => toggleRewardParticipant(person.id)} type="checkbox" />
                    <span>{participantAvatar(person)}</span>
                    <strong>{person.name}</strong>
                  </label>
                ))}
              </div>
              <button type="submit">Добавить награду</button>
            </form>
          </section>
        </section>
)}
