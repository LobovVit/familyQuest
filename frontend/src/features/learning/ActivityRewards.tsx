import { useEffect, useState } from 'react'
import { useRuntime } from '../../application/runtime'
import type { ActivityReward } from '../../domain/learning'
import '../family/family.css'
export function ActivityRewards() {
 const { gateway } = useRuntime()
 const [items, setItems] = useState<ActivityReward[]>([]), [error, setError] = useState(''), [loading, setLoading] = useState(true)
 const [attempt, setAttempt] = useState(0)
 useEffect(() => { let active = true; setLoading(true); setError(''); void gateway.activityRewards().then(v => { if (active) setItems(v) }).catch(e => { if (active) setError(e instanceof Error ? e.message : 'Не удалось загрузить награды') }).finally(() => { if (active) setLoading(false) }); return () => { active = false } }, [gateway, attempt])
 return <section className="family-life"><h1>Мои звёзды и улыбки</h1><p>Эти награды уже включены в рейтинги дня, недели и месяца вместе с наградами за дела.</p><div className="family-callout"><p>🔢 Математика: 1 / 2 / 3 ⭐ за верный ответ на лёгком / среднем / сложном уровне; в столбик — 3 ⭐.</p><p>🏃 Спорт: 10 ⭐ + 2 🙂 за занятия, один раз на участника в день.</p><p>🌱 Привычка: 5 ⭐ + 1 🙂 за участие по расписанию, один раз в день за каждую привычку.</p><p>🧭 Приключение: 20 ⭐ + 3 🙂 каждому назначенному участнику после всех шагов; если участники не назначены — тем, кто выполнял шаги.</p></div>{loading ? <p role="status">Загружаем награды…</p> : error ? <p role="alert">{error}<button onClick={() => setAttempt(v => v + 1)}>Повторить</button></p> : <><h2>Последние начисления</h2>{!items.length && <p>Первое достижение впереди. Запиши тренировку, отметь привычку или реши пример.</p>}<div className="family-grid">{items.map(r => <article className="family-card" key={`${r.source}/${r.sourceKey}/${r.participantId}`}><h3>{r.title}</h3><p>{r.date} · {{ math: 'Математика', habit: 'Привычка', adventure: 'Приключение', sport: 'Спорт' }[r.source]}</p><strong>+{r.stars} ⭐ {r.smiles > 0 && `+${r.smiles} 🙂`}</strong></article>)}</div>{items.length === 200 && <p>Показаны последние 200 начислений. Полная история сохранена.</p>}</>}</section>
}
