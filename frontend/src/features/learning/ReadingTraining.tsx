import { allowsReading, type LearningPolicy } from '../../domain/subscription'
import { useRuntime } from '../../application/runtime'
import type { ActivityReward } from '../../domain/learning'
import { useEffect, useRef, useState } from 'react'
import { readingLesson, readingModes, readingTextCount, readingStars, type ReadingMode } from '../../domain/reading'
import '../family/family.css'
import './reading.css'

export function ReadingTraining({ onReward, useAgePolicy = false }: { onReward?: () => void; useAgePolicy?: boolean }) {
  const { gateway } = useRuntime()
 const [policy,setPolicy]=useState<LearningPolicy|null>(null)
 useEffect(()=>{if(!useAgePolicy)return;let active=true;void gateway.learningPolicy().then(v=>{if(active)setPolicy(v)}).catch(()=>{if(active)setError('Не удалось загрузить настройки обучения. Открой раздел ещё раз.')});return()=>{active=false}},[gateway,useAgePolicy])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [reward, setReward] = useState<ActivityReward | null>(null)
  const completionID = useRef('')
  const lock = useRef(false)
  const mounted = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  const [mode, setMode] = useState<ReadingMode | null>(null)
  const [round, setRound] = useState(0)
  const [index, setIndex] = useState(0)
  const [syllable, setSyllable] = useState(0)
  const [whole, setWhole] = useState(false)
  const heading = useRef<HTMLHeadingElement>(null)
  const cards = readingLesson(mode ?? 'phrases', round)
  const card = cards[index]
  const count = card?.words.flat().length ?? 0

  async function reset(nextMode: ReadingMode | null, nextRound = round) {
    if (lock.current) return
    const id = nextMode ? crypto.randomUUID().replaceAll('-', '') : ''
    if (nextMode && useAgePolicy) {
      lock.current=true;setBusy(true);setError('')
      try { await gateway.startReading(id,nextMode) }
      catch(e) { if(mounted.current)setError(e instanceof Error?e.message:'Не удалось начать занятие');return }
      finally { lock.current=false;if(mounted.current)setBusy(false) }
      if(!mounted.current)return
    }
    completionID.current=id
    setReward(null); setError('')
    setMode(nextMode); setRound(nextRound); setIndex(0); setSyllable(0); setWhole(false)
  }
  async function next() {
    if (lock.current || !mode) return
    if (index < cards.length - 1) {
      setIndex(value => value + 1); setSyllable(0); setWhole(false)
      heading.current?.focus()
      return
    }
    lock.current = true; setBusy(true); setError('')
    try {
      const saved = await gateway.completeReading(completionID.current, mode)
      if (!mounted.current) return
      setReward(saved); setIndex(cards.length); setSyllable(0); setWhole(false)
      onReward?.()
      heading.current?.focus()
    } catch (e) {
      if (mounted.current) setError(e instanceof Error ? e.message : 'Не удалось сохранить занятие. Попробуй ещё раз — звёзды не удвоятся.')
    } finally {
      lock.current = false
      if (mounted.current) setBusy(false)
    }
  }

  return <section className="family-life reading-training" aria-label="Чтение">
    <header className="family-row"><div><p className="eyebrow">Учимся читать · шаг за шагом</p><h1>Чтение 📖</h1><p>Читай вслух в своём темпе. Можно позвать взрослого на помощь.</p></div>
      {mode && <button disabled={busy} onClick={() => reset(null)}>К выбору занятия</button>}
    </header>
    {error && <p role="alert" className="notice">{error}</p>}
    {!mode ? <>
      <h2>Выбери уровень</h2>{useAgePolicy && !policy && <p role="status">Загружаем твои настройки…</p>}
      <div className="reading-modes">{readingModes.filter(item=>!useAgePolicy||(policy&&allowsReading(policy,item.id))).map(item => <button disabled={busy} key={item.id} onClick={() => reset(item.id)}>
        <span aria-hidden="true">{item.icon}</span><strong>{item.title}</strong><small>{item.description}</small><small>{readingTextCount(item.id)} текстов · {readingTextCount(item.id) / 6} занятий</small><small>До {readingStars[item.id]} ⭐ за занятие</small>
      </button>)}</div>
      <p>До 30 ⭐ в день за чтение. В занятии 6 карточек. Без спешки — любую можно прочитать ещё раз.</p>
      <p className="reading-note">Для взрослого: слушайте ребёнка и помогайте при необходимости. Кнопка «Прочитано» отмечает прохождение карточки, а не проверяет произношение. За все 6 карточек начисляются звёзды по отметке о завершении. Награда сохраняется в рейтинге; после выхода из раздела карточки начнутся заново.</p>
    </> : <>
      <div className="reading-progress"><progress aria-label="Прочитанные карточки" max={cards.length} value={index} /><span>{index} из {cards.length}</span></div>
      <h2 ref={heading} tabIndex={-1}>{index === cards.length ? 'Занятие завершено! 🎉' : `${readingModes.find(item => item.id === mode)?.title} · карточка ${index + 1}`}</h2>
      {card ? <>
        <p id="reading-help">{whole ? 'Теперь прочитай целиком.' : 'Читай выделенный слог. Нажми на другой слог, чтобы вернуться к нему.'}</p>
        <div className="reading-paper" aria-describedby="reading-help">
          {card.words.map((parts, wordIndex) => {
            const offset = card.words.slice(0, wordIndex).reduce((sum, word) => sum + word.length, 0)
            return <span className="reading-word" key={wordIndex}>{whole ? parts.join('') : parts.map((part, partIndex) => <span className="reading-part" key={partIndex}>
              {partIndex > 0 && <span className="reading-divider" aria-hidden="true">·</span>}
              <button aria-label={`Слог ${offset + partIndex + 1}: ${part}`} aria-pressed={syllable === offset + partIndex} onClick={() => setSyllable(offset + partIndex)}>{part}</button>
            </span>)}</span>
          })}
        </div>
        <div className="reading-actions">
          {!whole && count > 1 && <button disabled={syllable === count - 1} onClick={() => setSyllable(value => Math.min(count - 1, value + 1))}>Следующий слог →</button>}
          {count > 1 && <button aria-pressed={whole} onClick={() => setWhole(value => !value)}>{whole ? 'По слогам' : 'Показать целиком'}</button>}
          <button className="family-primary" disabled={busy} onClick={() => { void next() }}>{busy ? 'Сохраняем награду…' : 'Прочитано ✓'}</button>
        </div>
      </> : <div className="reading-finish" role="status"><span aria-hidden="true">🌟</span><strong>{reward && reward.stars > 0 ? `+${reward.stars} ⭐ добавлено в твой рейтинг` : 'Сегодня все 30 ⭐ за чтение уже заработаны!'}</strong><p>Все 6 карточек пройдены. Можно отдохнуть или почитать ещё!</p><button className="family-primary" onClick={() => reset(mode, round + 1)}>Ещё 6 карточек</button><button onClick={() => reset(null, round + 1)}>Выбрать другое занятие</button></div>}
    </>}
  </section>
}
