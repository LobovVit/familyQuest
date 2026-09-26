import { readingMaterial as material } from './reading/material'

export type ReadingMode = 'phrases' | 'sentences' | 'advanced'
export type ReadingCard = { words: string[][] }
export const readingModes: { id: ReadingMode; title: string; description: string; icon: string }[] = [
  { id: 'phrases', title: 'Уровень 1 · Простые фразы', description: '2–4 слова: начинаем с коротких фраз', icon: '🌱' },
  { id: 'sentences', title: 'Уровень 2 · Предложения', description: '5–7 слов: читаем более длинные предложения', icon: '🌷' },
  { id: 'advanced', title: 'Уровень 3 · Сложные предложения', description: '8–12 слов: читаем внимательно, делаем паузы', icon: '🌳' },
]


export const readingTextCount = (mode: ReadingMode): number => material[mode].length

export function readingLesson(mode: ReadingMode, round: number): ReadingCard[] {
  const cards = material[mode]
  return Array.from({ length: 6 }, (_, index) => ({
    words: cards[(round * 6 + index) % cards.length].split(' ').map(word => word.split('-')),
  }))
}
