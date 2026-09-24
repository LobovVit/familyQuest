// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { ReadingTraining } from './ReadingTraining'
import { readingLesson, readingModes } from '../../domain/reading'
afterEach(cleanup)
it('allows following syllables and switching to whole words', () => {
  render(<ReadingTraining />)
  fireEvent.click(screen.getByRole('button', { name: /Уровень 1/ }))
  expect(screen.getByRole('button', { name: 'Слог 1: У' }).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', { name: 'Следующий слог →' }))
  expect(screen.getByRole('button', { name: 'Слог 2: ма' }).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', { name: 'Показать целиком' }))
  expect(screen.getByText('мамы')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'По слогам' }))
  fireEvent.click(screen.getByRole('button', { name: 'Слог 1: У' }))
  expect(screen.getByRole('button', { name: 'Слог 1: У' }).getAttribute('aria-pressed')).toBe('true')
})
it('finishes six cards, offers new material and resets on returning to the menu', () => {
  render(<ReadingTraining />)
  fireEvent.click(screen.getByRole('button', { name: /Уровень 1/ }))
  for (let i = 0; i < 6; i++) fireEvent.click(screen.getByRole('button', { name: 'Прочитано ✓' }))
  expect(screen.getByRole('heading', { name: 'Занятие завершено! 🎉' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Прочитано ✓' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Ещё 6 карточек' }))
  expect(screen.getByRole('button', { name: 'Слог 1: Ма' })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'К выбору занятия' }))
  fireEvent.click(screen.getByRole('button', { name: /Уровень 3/ }))
  expect(screen.getByRole('progressbar').getAttribute('value')).toBe('0')
})
it('keeps one vowel per syllable while allowing consonant-only prepositions', () => {
  for (const mode of readingModes) for (let round = 0; round < 8; round++) {
    const lesson = readingLesson(mode.id, round)
    expect(lesson).toHaveLength(6)
    for (const card of lesson) for (const word of card.words) for (const syllable of word) {
      if (word.length === 1 && ['в', 'к', 'с'].includes(syllable.toLowerCase())) continue
      expect(syllable.match(/[аеёиоуыэюя]/gi)).toHaveLength(1)
    }
  }
})

it('increases sentence length across all three levels', () => {
  const ranges = [[2, 4], [5, 7], [8, 12]]
  readingModes.forEach((mode, level) => {
    for (let round = 0; round < 2; round++) for (const card of readingLesson(mode.id, round)) {
      expect(card.words.length).toBeGreaterThanOrEqual(ranges[level][0])
      expect(card.words.length).toBeLessThanOrEqual(ranges[level][1])
    }
  })
})
