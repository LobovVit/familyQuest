import { useState } from 'react'
import { activities, matchesActivity, type Activity, type ActivityFilters } from './library'
export function ActivityLibrary({ onChoose }: { onChoose: (activity: Activity) => void }) {
  const [filters, setFilters] = useState<ActivityFilters>({ minutes: 30, age: 6, place: 'any', energy: 'any', budget: 'free', query: '' })
  const patch = (fields: Partial<ActivityFilters>) => setFilters(f => ({ ...f, ...fields }))
  const matched = activities.filter(a => matchesActivity(a, filters))
  return <section className="family-section">
    <div><h2>Чем займёмся сегодня?</h2><p className="family-muted">Выберите условия, затем распределите роли по возрасту и интересам.</p></div>
    <div className="family-filters">
      <label>Есть времени<select value={filters.minutes} onChange={e => patch({ minutes: Number(e.target.value) })}>{[10, 15, 20, 30, 60].map(v => <option key={v} value={v}>{v} минут</option>)}</select></label>
      <label>Возраст младшего<input type="number" min={3} max={18} value={filters.age} onChange={e => patch({ age: Number(e.target.value) })} /></label>
      <label>Где<select value={filters.place} onChange={e => patch({ place: e.target.value })}><option value="any">Где угодно</option><option value="home">Дома</option><option value="outside">На улице</option></select></label>
      <label>Настроение<select value={filters.energy} onChange={e => patch({ energy: e.target.value })}><option value="any">Любое</option><option value="quiet">Хочется спокойствия</option><option value="active">Хочется движения</option></select></label>
      <label>Бюджет<select value={filters.budget} onChange={e => patch({ budget: e.target.value })}><option value="free">Без покупок</option><option value="any">Можно купить материалы</option></select></label>
      <label>Интерес или материал<input placeholder="Например, бумага" value={filters.query} onChange={e => patch({ query: e.target.value })} /></label>
    </div>
    <p role="status" className="family-muted">Подходит занятий: {matched.length}</p>
    {!matched.length && <div className="family-empty"><h3>Пока нет точного совпадения</h3><p>Попробуйте увеличить время или изменить условия. Можно придумать своё приключение.</p></div>}
    <div className="family-grid">{matched.map(a => <article className="family-card" key={a.id}><div className="family-row"><span className="family-tag">{a.value}</span><small>{a.age}+ · {a.minutes} мин</small></div><h3>{a.title}</h3><p><strong>Подготовка:</strong> {a.materials}</p><ol>{a.steps.map(step => <li key={step}>{step}</li>)}</ol><details><summary>Роли и вопрос для обсуждения</summary><p>Младшим: {a.younger}</p><p>Старшим: {a.older}</p><p>{a.question}</p></details><button onClick={() => onChoose(a)}>Выбрать и распределить роли</button></article>)}</div>
  </section>
}
