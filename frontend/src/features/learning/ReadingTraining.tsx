import { useRef, useState } from 'react'
import { readingLesson, readingModes, type ReadingMode } from '../../domain/reading'
import '../family/family.css'
import './reading.css'

export function ReadingTraining() {
  const [mode, setMode] = useState<ReadingMode | null>(null)
  const [round, setRound] = useState(0)
  const [index, setIndex] = useState(0)
  const [syllable, setSyllable] = useState(0)
  const [whole, setWhole] = useState(false)
  const heading = useRef<HTMLHeadingElement>(null)
  const cards = readingLesson(mode ?? 'phrases', round)
  const card = cards[index]
  const count = card?.words.flat().length ?? 0

  function reset(nextMode: ReadingMode | null, nextRound = round) {
    setMode(nextMode); setRound(nextRound); setIndex(0); setSyllable(0); setWhole(false)
  }
  function next() {
    setIndex(value => value + 1); setSyllable(0); setWhole(false)
    heading.current?.focus()
  }

  return <section className="family-life reading-training" aria-label="Чтение">
    <header className="family-row"><div><p className="eyebrow">Учимся читать · шаг за шагом</p><h1>Чтение 📖</h1><p>Читай вслух в своём темпе. Можно позвать взрослого на помощь.</p></div>
      {mode && <button onClick={() => reset(null)}>К выбору занятия</button>}
    </header>
    {!mode ? <>
      <h2>Выбери уровень</h2>
      <div className="reading-modes">{readingModes.map(item => <button key={item.id} onClick={() => reset(item.id)}>
        <span aria-hidden="true">{item.icon}</span><strong>{item.title}</strong><small>{item.description}</small>
      </button>)}</div>
      <p>В занятии 6 карточек. Без спешки — любую можно прочитать ещё раз.</p>
      <p className="reading-note">Для взрослого: слушайте ребёнка и помогайте при необходимости. Кнопка «Прочитано» отмечает прохождение карточки, а не проверяет произношение. Звёзды не начисляются; после выхода из раздела занятие начнётся заново.</p>
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
          <button className="family-primary" onClick={next}>Прочитано ✓</button>
        </div>
      </> : <div className="reading-finish" role="status"><span aria-hidden="true">🌟</span><p>Все 6 карточек пройдены. Можно отдохнуть или почитать ещё!</p><button className="family-primary" onClick={() => reset(mode, round + 1)}>Ещё 6 карточек</button><button onClick={() => reset(null, round + 1)}>Выбрать другое занятие</button></div>}
    </>}
  </section>
}
