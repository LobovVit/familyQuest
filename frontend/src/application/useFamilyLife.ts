import { useCallback, useEffect, useRef, useState } from 'react'
import { useRuntime } from './runtime'
import type { FamilyAction, FamilyDraft, FamilyEntry, FamilyKind } from '../domain/family'

export function useFamilyLife(onProgress?: () => void) {
  const { gateway } = useRuntime()
  const [entries, setEntries] = useState<FamilyEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const active = useRef(true)
  const lock = useRef(false)
  const revision = useRef(0)
  const invalidate = useCallback(() => { revision.current++ }, [])
  const refresh = useCallback(async () => {
    const request = ++revision.current
    if (active.current) { setLoading(true); setError('') }
    try {
      const values = await gateway.familyEntries()
      if (active.current && request === revision.current) setEntries(values)
    } catch (error) {
      if (active.current && request === revision.current) {
        setError(error instanceof Error ? error.message : 'Не удалось загрузить раздел')
        throw error
      }
    } finally {
      if (active.current && request === revision.current) setLoading(false)
    }
  }, [gateway])
  useEffect(() => {
    active.current = true
    void refresh().catch(() => { /* refresh exposes current errors in state */ })
    return () => { active.current = false; invalidate() }
  }, [refresh, invalidate])
  async function mutate(operation: () => Promise<FamilyEntry>): Promise<boolean> {
    if (lock.current) return false
    lock.current = true; revision.current++; setBusy(true); setError('')
    try {
      const saved = await operation()
      revision.current++
      if (active.current) setEntries(items => [saved, ...items.filter(e => e.id !== saved.id)].sort((a, b) => b.id - a.id))
      if (active.current) onProgress?.()
      return true
    } catch (e) {
      let message = e instanceof Error ? e.message : 'Не удалось сохранить'
      if (e instanceof Error && 'status' in e && e.status === 409) {
        message = 'Карточка уже изменилась. Данные обновлены; проверьте их и повторите действие.'
        try { await refresh() } catch { message += ' Не удалось загрузить свежие данные; обновите страницу.' }
      }
      if (active.current) setError(message)
      return false
    } finally { lock.current = false; if (active.current) { setBusy(false); setLoading(false) } }
  }
  return { entries, loading, busy, error, setError, refresh,
    save: (kind: FamilyKind, draft: FamilyDraft, entry?: FamilyEntry) => entry
      ? mutate(() => gateway.editFamily(entry.id, entry.version, draft))
      : mutate(() => gateway.createFamily(kind, draft)),
    act: (entry: FamilyEntry, action: FamilyAction) => mutate(() => gateway.familyAction(entry.id, entry.version, action)),
  }
}
