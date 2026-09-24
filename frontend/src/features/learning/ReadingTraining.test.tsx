// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { ReadingTraining } from './ReadingTraining'
import { readingLesson, readingModes } from '../../domain/reading'
afterEach(cleanup)
it('allows following syllables and switching to whole words', () => {
  render(<ReadingTraining />)
  fireEvent.click(screen.getByRole('button', { name: /Слова/ }))
  expect(screen.getByRole('button', { name: 'Слог 1: ма' }).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', { name: 'Следующий слог →' }))
  expect(screen.getByRole('button', { name: 'Слог 2: ма' }).getAttribute('aria-pressed')).toBe('true')
  fireEvent.click(screen.getByRole('button', { name: 'Показать целиком' }))
  expect(screen.getByText('мама')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'По слогам' }))
  fireEvent.click(screen.getByRole('button', { name: 'Слог 1: ма' }))
  expect(screen.getByRole('button', { name: 'Слог 1: ма' }).getAttribute('aria-pressed')).toBe('true')
})
it('finishes six cards, offers new material and resets on returning to the menu', () => {
  render(<ReadingTraining />)
  fireEvent.click(screen.getByRole('button', { name: /^Слоги/ }))
  for (let i = 0; i < 6; i++) fireEvent.click(screen.getByRole('button', { name: 'Прочитано ✓' }))
  expect(screen.getByRole('heading', { name: 'Занятие завершено! 🎉' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Прочитано ✓' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Ещё 6 карточек' }))
  expect(screen.getByRole('button', { name: 'Слог 1: ну' })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'К выбору занятия' }))
  fireEvent.click(screen.getByRole('button', { name: /Фразы/ }))
  expect(screen.getByRole('progressbar').getAttribute('value')).toBe('0')
})
it('keeps one vowel per syllable in every lesson and cycles through complete cards', () => {
  for (const mode of readingModes) for (let round = 0; round < 8; round++) {
    const lesson = readingLesson(mode.id, round)
    expect(lesson).toHaveLength(6)
    for (const card of lesson) for (const syllable of card.words.flat()) {
      expect(syllable.match(/[аеёиоуыэюя]/gi)).toHaveLength(1)
    }
  }
})
