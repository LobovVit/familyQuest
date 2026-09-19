export async function readMedia(file: File, kind: 'photo' | 'audio'): Promise<string> {
  if (kind === 'audio') {
    if (file.size > 500000) throw new Error('Аудио должно быть меньше 500 КБ. Выберите короткую запись.')
    const type = file.type === 'audio/x-m4a' ? 'audio/mp4' : file.type === 'audio/x-wav' ? 'audio/wav' : file.type
    if (!['audio/mpeg', 'audio/wav', 'audio/ogg', 'audio/webm', 'audio/mp4'].includes(type)) throw new Error('Выберите MP3, WAV, OGG, WebM или M4A')
    return readData(new Blob([file], { type }))
  }
  if (!['image/jpeg', 'image/png'].includes(file.type) || file.size > 15000000) throw new Error('Выберите JPEG или PNG до 15 МБ')
  const image = await createImageBitmap(file)
  try {
    const ratio = Math.min(1, 960 / Math.max(image.width, image.height))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(image.width * ratio)); canvas.height = Math.max(1, Math.round(image.height * ratio))
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('Браузер не поддерживает обработку фото')
    ctx.fillStyle = '#fff'; ctx.fillRect(0, 0, canvas.width, canvas.height); ctx.drawImage(image, 0, 0, canvas.width, canvas.height)
    const data = canvas.toDataURL('image/jpeg', 0.78)
    if (data.length > 666000) throw new Error('Фото слишком большое; выберите изображение меньшего размера')
    return data
  } finally { image.close() }
}
function readData(file: Blob): Promise<string> {
  return new Promise((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = () => reject(new Error('Не удалось прочитать файл')); reader.readAsDataURL(file) })
}
