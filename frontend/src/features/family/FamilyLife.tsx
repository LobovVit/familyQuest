import { useRef, useState } from 'react'
import type { Participant } from '../../domain/models'
import { useFamilyLife } from '../../application/useFamilyLife'
import { emptyDraft, entryDraft, gardenMilestones, isDue, kindIcons, kindLabels, type FamilyDraft, type FamilyEntry, type FamilyKind } from '../../domain/family'
import { ActivityLibrary } from './ActivityLibrary'
import { activityDraft } from './library'
import { FamilyEditor } from './FamilyEditor'
import { FamilyCard } from './FamilyCard'
import './family.css'

type View = 'today' | 'activities' | 'garden' | FamilyKind
type Editor = { kind: FamilyKind; draft: FamilyDraft; entry?: FamilyEntry }
const views: Array<{ id: View; label: string; icon: string }> = [{ id: 'today', label: 'Сегодня вместе', icon: '☀️' }, { id: 'activities', label: 'Чем займёмся?', icon: '🎲' }, ...Object.entries(kindLabels).filter(([id]) => id !== 'sport').map(([id, label]) => ({ id: id as FamilyKind, label, icon: kindIcons[id as FamilyKind] })), { id: 'garden', label: 'Наш сад', icon: '🌳' }]
export function FamilyLife({ current, participants, date }: { current: Participant; participants: Participant[]; date: string }) {
  const data = useFamilyLife()
  const [view, setView] = useState<View>('today')
  const [showArchive, setShowArchive] = useState(false)
  const [editor, setEditor] = useState<Editor | null>(null)
  const [search, setSearch] = useState('')
  const editorAnchor = useRef<HTMLDivElement>(null)
  const family = participants.filter(p => p.active && (p.role === 'parent' || p.role === 'child'))
  const milestones = gardenMilestones(data.entries)
  const due = data.entries.filter(e => isDue(e, date))
  const memories = data.entries.filter(e => !e.archived && e.kind === 'memory' && e.date < date && e.date.slice(5) === date.slice(5))
  const pending = data.entries.filter(e => e.kind === 'proposal' && !e.archived && !e.approved)
  const newerEntry = editor?.entry && data.entries.find(e => e.id === editor.entry?.id && e.version !== editor.entry?.version)
  const selected = data.entries.filter(e => e.kind === view && e.archived === showArchive && `${e.title} ${e.description} ${e.value}`.toLocaleLowerCase('ru').includes(search.toLocaleLowerCase('ru')))
  function open(kind: FamilyKind, draft = emptyDraft(date), entry?: FamilyEntry) {
    setEditor({ kind, draft, entry }); data.setError('')
    requestAnimationFrame(() => editorAnchor.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
  }
  function repeat(e: FamilyEntry) {
    const draft = { ...entryDraft(e), date, photo: '', audio: '', agreement: '', nextActivity: '' }
    if (e.kind === 'memory' || e.kind === 'council') { draft.steps = [{ title: 'Вместе выбрать план и распределить роли', participantId: 0 }]; draft.description = `Продолжение: ${e.title}\n${e.kind === 'council' ? e.nextActivity : e.description}` }
    open(['memory', 'council', 'proposal'].includes(e.kind) ? 'adventure' : e.kind, draft)
  }
  function card(e: FamilyEntry) { return <FamilyCard key={e.id} entry={e} current={current} participants={participants} date={date} busy={data.busy} onAction={action => data.act(e, action)} onEdit={() => open(e.kind, entryDraft(e), e)} onRepeat={() => repeat(e)} onMemory={() => open('memory', { ...emptyDraft(date), title: e.title, value: e.value, participantIds: e.participantIds ?? [], description: '' })} onPlan={() => open('adventure', { ...emptyDraft(date), title: e.nextActivity.slice(0, 160), description: `Решение семейного совета «${e.title}»`, steps: [{ title: 'Подготовиться и распределить роли', participantId: 0 }] })} /> }
  return <section className="family-life" aria-label="Семейная жизнь">
    <header className="family-hero"><div><p className="eyebrow">FamilyQuest · вместе каждый день</p><h1>Маленькие дела.<br />Большие воспоминания.</h1><p>Пробуйте новое, заботьтесь друг о друге и создавайте свои традиции.</p><div className="family-row"><button className="family-primary" onClick={() => setView('activities')}>Подобрать занятие</button><button onClick={() => open('proposal', { ...emptyDraft(date), steps: [{ title: '', participantId: 0 }] })}>Предложить свою идею</button></div></div><div className="family-hero-garden" aria-label={`Достижений в саду: ${milestones.length}`}><span aria-hidden="true">🌱 🏡 🌳</span><strong>Добрых следов: {milestones.length}</strong><p>Каждый вклад важен.<br />После паузы можно продолжить.</p></div></header>
    <nav className="family-nav" aria-label="Разделы семейной жизни">{views.map(v => <button key={v.id} className={view === v.id ? 'active' : ''} aria-current={view === v.id ? 'page' : undefined} onClick={() => { setView(v.id); setShowArchive(false); setSearch('') }}>{v.icon} {v.label}</button>)}</nav>
    {data.error && <div className="notice" role="alert">{data.error}<button disabled={data.busy} onClick={() => { void data.refresh().then(() => data.setError('')).catch(() => data.setError('Не удалось обновить данные')) }}>Обновить данные</button></div>}
    <div ref={editorAnchor}>{newerEntry && <div className="family-callout"><p>Карточку изменили на другом устройстве. Перечитайте её перед редактированием; несохранённый текст заменится актуальными данными.</p><button onClick={() => open(newerEntry.kind, entryDraft(newerEntry), newerEntry)}>Перечитать карточку</button></div>}{editor && <FamilyEditor key={`${editor.entry?.id ?? 'new'}-${editor.entry?.version ?? 0}-${editor.kind}-${editor.draft.title}`} kind={editor.kind} initial={editor.draft} entry={editor.entry} participants={family} busy={data.busy} onClose={() => setEditor(null)} onSave={async draft => {
      const ok = await data.save(editor.kind, draft, editor.entry)
      if (ok) setView(editor.kind === 'adventure' && current.role === 'child' ? 'proposal' : editor.kind)
      return ok
    }} />}</div>
    {data.loading ? <p role="status">Загружаем семейные истории…</p> : <>
      {view === 'today' && <section className="family-section"><div className="family-row"><div><h2>Сегодня вместе</h2><p className="family-muted">{new Date(`${date}T12:00:00`).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', weekday: 'long' })} · выберите посильное</p></div><button onClick={() => open('habit')}>+ Семейная привычка</button></div>
        {pending.length > 0 && <div className="family-callout"><p>💡 Идей на обсуждение: {pending.length}. Каждый может предложить своё приключение.</p><button onClick={() => setView('proposal')}>Посмотреть идеи</button></div>}
        {due.length ? <div className="family-grid">{due.map(card)}</div> : <div className="family-empty"><span aria-hidden="true">☀️</span><h3>Есть место для чего-то хорошего</h3><p>Выберите занятие из библиотеки или начните одну маленькую привычку.</p><button onClick={() => setView('activities')}>Найти идею для семьи</button></div>}
        {memories.length > 0 && <><h2>В этот день раньше</h2><div className="family-grid">{memories.map(card)}</div></>}
        <div className="family-prompt"><h3>Вопрос для разговора</h3><p>Что сегодня порадовало тебя и чем мы можем помочь друг другу?</p><button onClick={() => open('council', { ...emptyDraft(date), title: 'Наш семейный совет', description: 'Что порадовало? Кого хочется поблагодарить? Что изменим? Что попробуем вместе?' })}>Начать семейный совет</button></div>
      </section>}
      {view === 'activities' && <ActivityLibrary onChoose={a => open('adventure', activityDraft(a, date))} />}
      {view === 'garden' && <section className="family-section"><div><h2>Наш общий сад</h2><p className="family-muted">Дни участия, завершённые приключения, освоенные навыки и воспоминания. Пропуски ничего не отнимают.</p></div><div className="family-stats"><div><strong>{milestones.length}</strong><span>сохранённых достижений</span></div><div><strong>{data.entries.filter(e => e.events.some(v => v.kind === 'repeat')).length}</strong><span>занятий хочется повторить</span></div><div><strong>{data.entries.filter(e => e.kind === 'habit' && !e.archived).length}</strong><span>привычек продолжаем</span></div></div>{milestones.length ? <div className="family-garden">{milestones.slice(0, 100).map(m => <div className="family-plant" key={m.key}><span aria-hidden="true">{m.icon}</span><strong>{m.title}</strong><small>{m.date}</small></div>)}</div> : <div className="family-empty"><span aria-hidden="true">🌱</span><h3>Первый росток впереди</h3><p>Отметьте участие в привычке, завершите приключение или сохраните воспоминание.</p></div>}{milestones.length > 100 && <p>Показаны последние 100 достижений. Вся история сохранена в карточках.</p>}</section>}
      {!['today', 'activities', 'garden'].includes(view) && <section className="family-section"><div className="family-row"><h2>{kindIcons[view as FamilyKind]} {kindLabels[view as FamilyKind]}</h2><button className="family-primary" onClick={() => open(view as FamilyKind)}>+ Добавить</button></div><div className="family-row"><label className="family-search">Найти карточку<input placeholder="Название, ценность или история" value={search} onChange={e => setSearch(e.target.value)} /></label><label className="family-checkbox"><input type="checkbox" checked={showArchive} onChange={e => setShowArchive(e.target.checked)} />Показать архив</label></div>{selected.length ? <div className="family-grid">{selected.map(card)}</div> : <div className="family-empty"><span aria-hidden="true">{kindIcons[view as FamilyKind]}</span><h3>{showArchive ? 'В архиве пока пусто' : search ? 'Ничего не найдено' : 'Начните с одной карточки'}</h3><p>{showArchive ? 'Архив сохраняет историю завершённых дел.' : 'Можно выбрать готовый вариант или придумать свой.'}</p></div>}</section>}
    </>}
    <footer className="family-footer">Семейное пространство · заметки и воспоминания видны участникам семьи. Фото и записи всегда по желанию.</footer>
  </section>
}
