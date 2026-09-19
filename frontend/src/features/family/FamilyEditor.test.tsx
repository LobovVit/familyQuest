// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FamilyEditor } from './FamilyEditor'
import { emptyDraft } from '../../domain/family'
afterEach(cleanup)
describe('family editor', () => {
  it('saves native time/date input and a selected habit template', async () => {
    const onSave = vi.fn().mockResolvedValue(true); const onClose = vi.fn()
    render(<FamilyEditor kind="habit" participants={[]} initial={emptyDraft('2026-09-19')} busy={false} onSave={onSave} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Читаем перед сном' }))
    fireEvent.input(screen.getByLabelText(/Напомнить в календаре/), { target: { value: '20:30' } })
    fireEvent.input(screen.getByLabelText('Начинаем с'), { target: { value: '2026-09-21' } })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
    expect(onSave.mock.calls[0][0]).toMatchObject({ title: 'Читаем перед сном', reminder: '20:30', date: '2026-09-21', easyVersion: 'Прочитать один абзац или рассмотреть одну иллюстрацию.' })
    expect(onClose).toHaveBeenCalledOnce()
  })
  it('requires a plan for an adventure and keeps the draft after a rejected save', async () => {
    const onSave = vi.fn().mockResolvedValue(false); const onClose = vi.fn()
    render(<FamilyEditor kind="adventure" participants={[]} initial={{ ...emptyDraft(), title: 'Наш план' }} busy={false} onSave={onSave} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.getByRole('alert').textContent).toContain('хотя бы один шаг')
    fireEvent.click(screen.getByRole('button', { name: '+ Добавить шаг' }))
    fireEvent.change(screen.getByLabelText('Шаг 1'), { target: { value: 'Выбрать книгу' } })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
    expect(onClose).not.toHaveBeenCalled()
  })
  it('allows a memory without photos or audio', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(<FamilyEditor kind="memory" participants={[]} initial={{ ...emptyDraft(), title: 'Смешная история' }} busy={false} onSave={onSave} onClose={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => expect(onSave).toHaveBeenCalledOnce())
    expect(onSave.mock.calls[0][0]).toMatchObject({ photo: '', audio: '' })
  })
})
