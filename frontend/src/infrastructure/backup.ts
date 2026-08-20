import { api, download } from './apiClient'

export async function downloadBackup(date:string): Promise<void> {
  const blob = await download('/api/backup')
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url; link.download = `familyquest-backup-${date}.json`; document.body.appendChild(link); link.click(); link.remove()
  URL.revokeObjectURL(url)
}
export async function restoreBackup(file:File): Promise<void> {
  await api('/api/backup', { method:'POST', body:await file.text() })
}
