// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { AccountLogin } from './AccountLogin'
import { loginNavigation } from './loginNavigation'

afterEach(() => { cleanup(); vi.restoreAllMocks(); window.history.replaceState(null, '', '/') })
function setup(registerFamily = vi.fn().mockResolvedValue([{ id: 1, name: 'Анна', role: 'parent', active: true }])) {
 const open = vi.spyOn(loginNavigation, 'open').mockImplementation(() => {})
 const runtime = { gateway: { restoreAccount: vi.fn().mockResolvedValue({ onboarding: true }), registerFamily }, session: { subscribe: () => () => {}, getParticipant: () => null } } as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso loginUrl="https://lovit.tech/login?product=familyquest" /></RuntimeContext.Provider>)
 return { registerFamily, open }
}
async function fill(confirm = '739281') {
 fireEvent.change(await screen.findByLabelText('Название семьи'), { target: { value: '  Соколовы  ' } })
 fireEvent.change(screen.getByLabelText('Ваше имя'), { target: { value: ' Анна ' } })
 fireEvent.change(screen.getByLabelText('PIN родителя'), { target: { value: '739281' } })
 fireEvent.change(screen.getByLabelText('Повторите PIN'), { target: { value: confirm } })
}
it('restores onboarding without redirecting and opens profile selection after creation', async () => {
 const { registerFamily, open } = setup()
 await fill()
 expect(open).not.toHaveBeenCalled()
 expect(screen.queryByRole('link', { name: 'Продолжить в Делодень' })).toBeNull()
 fireEvent.click(screen.getByRole('button', { name: 'Создать семью' }))
 await screen.findByLabelText('Семейный профиль')
 expect(registerFamily).toHaveBeenCalledWith({ name: 'Соколовы', parentName: 'Анна', pin: '739281' })
 expect(screen.queryByLabelText('Название семьи')).toBeNull()
 expect(screen.getByText('Семья создана. Выберите свой профиль и введите PIN.')).toBeTruthy()
 expect((screen.getByRole('button', { name: 'Открыть профиль' }) as HTMLButtonElement).disabled).toBe(true)
})
it('rejects mismatched PIN confirmation before submitting', async () => {
 const { registerFamily } = setup()
 await fill('739282')
 fireEvent.click(screen.getByRole('button', { name: 'Создать семью' }))
 expect((await screen.findByRole('alert')).textContent).toContain('не совпадают')
 expect(registerFamily).not.toHaveBeenCalled()
})
it('retains the entered names after a failure and permits a retry', async () => {
 const registerFamily = vi.fn().mockRejectedValueOnce(new Error('Попробуйте ещё раз')).mockResolvedValueOnce([{id:1,active:true,name:'Анна'}])
 setup(registerFamily)
 await fill()
 fireEvent.click(screen.getByRole('button', { name: 'Создать семью' }))
 expect((await screen.findByRole('alert')).textContent).toBe('Попробуйте ещё раз')
 expect((screen.getByLabelText('Название семьи') as HTMLInputElement).value).toBe('  Соколовы  ')
 fireEvent.click(screen.getByRole('button', { name: 'Создать семью' }))
 await screen.findByLabelText('Семейный профиль')
 expect(registerFamily).toHaveBeenCalledTimes(2)
})
it('blocks duplicate submits while the request is pending', async () => {
 let done!: (profiles: unknown[]) => void
 const registerFamily = vi.fn().mockReturnValue(new Promise(resolve => { done = resolve }))
 setup(registerFamily)
 await fill()
 const form = screen.getByRole('button', { name: 'Создать семью' }).closest('form')!
 fireEvent.submit(form); fireEvent.submit(form)
 expect(registerFamily).toHaveBeenCalledTimes(1)
 expect((screen.getByRole('button', { name: 'Создаём семью…' }) as HTMLButtonElement).disabled).toBe(true)
 done([{id:1,active:true,name:'Анна'}])
 await waitFor(() => expect(screen.queryByLabelText('Название семьи')).toBeNull())
})
