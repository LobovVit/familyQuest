// @vitest-environment jsdom
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ReadingTraining } from './ReadingTraining'
import { readingLesson, readingModes, readingTextCount } from '../../domain/reading'
const completeReading = vi.fn()
const runtime = { gateway: { completeReading } } as unknown as Runtime
function renderReading() { return render(<RuntimeContext.Provider value={runtime}><ReadingTraining /></RuntimeContext.Provider>) }
beforeEach(() => { completeReading.mockReset(); completeReading.mockResolvedValue({ stars: 6 }) })
afterEach(cleanup)
it('allows following syllables and switching to whole words', () => {
  renderReading()
  fireEvent.click(screen.getByRole('button', { name: /Уровень 1/ }))
  expect(screen.getByRole('button', { name: 'Слог 1: Ма' }).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', { name: 'Следующий слог →' }))
  expect(screen.getByRole('button', { name: 'Слог 2: ма' }).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', { name: 'Показать целиком' }))
  expect(screen.getByText('Мама')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'По слогам' }))
  fireEvent.click(screen.getByRole('button', { name: 'Слог 1: Ма' }))
  expect(screen.getByRole('button', { name: 'Слог 1: Ма' }).getAttribute('aria-pressed')).toBe('true')
})
it('finishes six cards, offers new material and resets on returning to the menu', async () => {
  renderReading()
  fireEvent.click(screen.getByRole('button', { name: /Уровень 1/ }))
  for (let i = 0; i < 6; i++) fireEvent.click(screen.getByRole('button', { name: 'Прочитано ✓' }))
  await waitFor(() => expect(screen.getByRole('heading', { name: 'Занятие завершено! 🎉' })).toBeTruthy())
  expect(completeReading).toHaveBeenCalledTimes(1)
  expect(screen.getByText('+6 ⭐ добавлено в твой рейтинг')).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Прочитано ✓' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Ещё 6 карточек' }))
  expect(screen.getByRole('button', { name: 'Слог 1: По' })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'К выбору занятия' }))
  fireEvent.click(screen.getByRole('button', { name: /Уровень 3/ }))
  expect(screen.getByRole('progressbar').getAttribute('value')).toBe('0')
})
it('keeps one vowel per syllable while allowing consonant-only prepositions', () => {
  for (const mode of readingModes) for (let round = 0; round < readingTextCount(mode.id) / 6; round++) {
    const lesson = readingLesson(mode.id, round)
    expect(lesson).toHaveLength(6)
    for (const card of lesson) for (const word of card.words) for (const syllable of word) {
      if (word.length === 1 && ['в', 'к', 'с'].includes(syllable.toLowerCase())) continue
      expect(syllable.match(/[аеёиоуыэюя]/gi)).toHaveLength(1)
    }
  }
})

it('increases sentence length across all three levels', () => {
  const ranges = [[5, 7], [8, 12], [13, 17]]
  readingModes.forEach((mode, level) => {
    for (let round = 0; round < readingTextCount(mode.id) / 6; round++) for (const card of readingLesson(mode.id, round)) {
      expect(card.words.length).toBeGreaterThanOrEqual(ranges[level][0])
      expect(card.words.length).toBeLessThanOrEqual(ranges[level][1])
    }
  })
})

it('offers 120 unique texts per level before repeating the first lesson', () => {
  for (const mode of readingModes) {
    expect(readingTextCount(mode.id)).toBe(120)
    const texts = Array.from({ length: 20 }, (_, round) => readingLesson(mode.id, round))
      .flat().map(card => card.words.map(word => word.join('')).join(' ').toLowerCase())
    expect(new Set(texts).size).toBe(120)
    expect(readingLesson(mode.id, 20)).toEqual(readingLesson(mode.id, 0))
    expect(texts.every(text => /^[а-яё]/i.test(text) && /[.!?]$/.test(text))).toBe(true)
  }
})

it('retries a failed completion with the same id and does not award unfinished lessons', async () => {
 completeReading.mockRejectedValueOnce(new Error('Сеть недоступна'))
 renderReading()
 fireEvent.click(screen.getByRole('button', { name: /Уровень 1/ }))
 for (let i = 0; i < 5; i++) fireEvent.click(screen.getByRole('button', { name: 'Прочитано ✓' }))
 expect(completeReading).not.toHaveBeenCalled()
 fireEvent.click(screen.getByRole('button', { name: 'Прочитано ✓' }))
 await screen.findByRole('alert')
 expect(screen.queryByText(/добавлено в твой рейтинг/)).toBeNull()
 fireEvent.click(screen.getByRole('button', { name: 'Прочитано ✓' }))
 await screen.findByText('+6 ⭐ добавлено в твой рейтинг')
 expect(completeReading.mock.calls[1]).toEqual(completeReading.mock.calls[0])
 expect(completeReading.mock.calls[0][0]).toMatch(/^[a-f0-9]{32}$/)
})
it('locks duplicate submissions and displays a capped reward honestly', async () => {
 let finish!: (value: { stars: number }) => void
 completeReading.mockImplementation(() => new Promise(resolve => { finish = resolve }))
 renderReading()
 fireEvent.click(screen.getByRole('button', { name: /Уровень 3/ }))
 for (let i = 0; i < 6; i++) fireEvent.click(screen.getByRole('button', { name: 'Прочитано ✓' }))
 fireEvent.click(screen.getByRole('button', { name: 'Сохраняем награду…' }))
 expect(completeReading).toHaveBeenCalledTimes(1)
 expect(completeReading.mock.calls[0][1]).toBe('advanced')
 finish({ stars: 0 })
 await screen.findByText('Сегодня все 30 ⭐ за чтение уже заработаны!')
})
