import { useEffect, useState } from 'react'
import './mcp.css'
import { useRuntime } from '../../application/runtime'
import type { MCPGrant } from '../../domain/mcp'
import type { Participant } from '../../domain/models'
import { roleLabels } from '../shared/labels'

export function MCPConnections({ participants, confirm }: { participants: Participant[]; confirm: () => Promise<string | null> }) {
 const { gateway } = useRuntime()
 const [items, setItems] = useState<MCPGrant[]>([])
 const [target, setTarget] = useState('')
 const [name, setName] = useState('')
 const [readOnly, setReadOnly] = useState(true)
 const [token, setToken] = useState('')
 const [error, setError] = useState('')
 const [busy, setBusy] = useState(false)
 const [loading, setLoading] = useState(true)
 const [attempt, setAttempt] = useState(0)
 const endpoint = `${window.location.origin}/api/mcp`
 useEffect(() => {
  let active = true
  setLoading(true)
  void gateway.mcpGrants().then(v => { if (active) { setItems(v); setError('') } }).catch(e => { if (active) setError(e instanceof Error ? e.message : 'Не удалось загрузить подключения') }).finally(() => { if (active) setLoading(false) })
  return () => { active = false }
 }, [gateway, attempt])
 async function create(event: React.FormEvent) {
  event.preventDefault()
  if (busy || !target || !name.trim()) return
  setBusy(true); setError(''); setToken('')
  try {
   const proof = await confirm()
   if (!proof) return
   const result = await gateway.createMCPGrant({ participantId: Number(target), name: name.trim(), readOnly }, proof)
   setItems(v => [result.grant, ...v]); setToken(result.token); setName('')
  } catch (e) { setError(e instanceof Error ? e.message : 'Не удалось создать подключение') }
  finally { setBusy(false) }
 }
 async function revoke(id: string) {
  if (busy) return
  setBusy(true); setError('')
  try { await gateway.revokeMCPGrant(id); setItems(v => v.filter(x => x.id !== id)); setToken('') }
  catch (e) { setError(e instanceof Error ? e.message : 'Не удалось отключить подключение') }
  finally { setBusy(false) }
 }
 return <section className="panel mcp-connections"><h2>ИИ-подключения · MCP</h2>
  <p>Родитель выдаёт агенту ключ конкретного профиля на 90 дней. Права определяются ролью: ребёнок отмечает свои дела, родитель также подтверждает чужие, школа читает результаты планера. Смена PIN отключает ключи профиля.</p>
  <p>Адрес сервера: <code>{endpoint}</code> · транспорт Streamable HTTP · заголовок <code>Authorization: Bearer КЛЮЧ</code>.</p>
  {loading && <p role="status">Загружаем подключения…</p>}
  {error && <p role="alert">{error} <button disabled={busy} onClick={() => setAttempt(v => v + 1)}>Повторить</button></p>}
  <form onSubmit={e => { void create(e) }} className="mcp-form">
   <label>Профиль для ИИ<select required value={target} disabled={busy} onChange={e => { setTarget(e.target.value); setToken('') }}><option value="">Выберите профиль</option>{participants.filter(p => p.active).map(p => <option key={p.id} value={p.id}>{p.name} · {roleLabels[p.role]}</option>)}</select></label>
   <label>Название подключения<input required maxLength={80} value={name} disabled={busy} onChange={e => setName(e.target.value)} placeholder="Например, домашний помощник" /></label>
   <label className="mcp-readonly"><input type="checkbox" checked={readOnly} disabled={busy} onChange={e => setReadOnly(e.target.checked)} />Только чтение результатов</label>
   <button disabled={busy || loading || !target || !name.trim()} type="submit">Создать ключ MCP</button>
  </form>
  {token && <div role="status"><p>Сохраните ключ: он показывается один раз. Передавайте его только доверенному агенту. Агент сможет читать данные выбранного профиля и выполнять разрешённые действия.</p><label>Новый ключ MCP<textarea readOnly value={token} rows={4} style={{ width: '100%', overflowWrap: 'anywhere' }} /></label><button onClick={() => setToken('')}>Скрыть ключ</button></div>}
  {!loading && !error && !items.length && <p>Пока нет подключений.</p>}
  <div className="family-grid">{items.map(g => <article className="family-card" key={g.id}><h3>{g.name}</h3><p>{participants.find(p => p.id === g.participantId)?.name ?? `Профиль ${g.participantId}`} · {g.readOnly ? 'Только чтение' : 'Действия в пределах роли'}</p><p>До {new Date(g.expiresAt).toLocaleDateString('ru-RU')}</p><button disabled={busy} onClick={() => { void revoke(g.id) }}>Отключить {g.name}</button></article>)}</div>
 </section>
}
