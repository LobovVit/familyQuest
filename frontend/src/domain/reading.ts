export type ReadingMode = 'syllables' | 'words' | 'phrases'
export type ReadingCard = { words: string[][] }
export const readingModes: { id: ReadingMode; title: string; description: string; icon: string }[] = [
  { id: 'syllables', title: 'Слоги', description: 'Ма, мо, му — читаем понемногу', icon: '🌱' },
  { id: 'words', title: 'Слова', description: 'Соединяем слоги в слова', icon: '🌷' },
  { id: 'phrases', title: 'Фразы', description: 'Читаем несколько слов вместе', icon: '🌳' },
]

// Explicit syllables keep the learning material independent of automatic hyphenation.
// Явные слоги исключают ошибки автоматического переноса в учебном материале.
const material: Record<ReadingMode, string[]> = {
  syllables: ['ма', 'мо', 'му', 'ми', 'на', 'но', 'ну', 'ни', 'ла', 'ло', 'лу', 'ли', 'ра', 'ро', 'ру', 'ри', 'са', 'со'],
  words: ['ма-ма', 'па-па', 'ра-ма', 'ла-па', 'ли-па', 'лу-на', 'ли-са', 'со-ва', 'ко-за', 'ва-та', 'но-га', 'ру-ка', 'ма-ли-на', 'ма-ши-на', 'па-на-ма', 'ба-на-ны', 'ку-би-ки', 'мо-ло-ко'],
  phrases: ['У ма-мы ма-ли-на.', 'У па-пы па-на-ма.', 'Ли-са у ли-пы.', 'У ко-зы но-ги.', 'У со-вы ла-пы.', 'На не-бе лу-на.', 'Ма-ма до-ма.', 'Па-па до-ма.', 'У Ни-ны ба-на-ны.', 'У Ди-мы ку-би-ки.', 'У Ми-лы мо-ло-ко.', 'У Ро-мы ма-ши-на.'],
}

export function readingLesson(mode: ReadingMode, round: number): ReadingCard[] {
  const cards = material[mode]
  return Array.from({ length: 6 }, (_, index) => ({
    words: cards[(round * 6 + index) % cards.length].split(' ').map(word => word.split('-')),
  }))
}
