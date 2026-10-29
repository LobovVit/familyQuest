// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { FamilySubscription } from './FamilySubscription'

afterEach(cleanup)
function setup(renameFamily = vi.fn().mockResolvedValue({name: 'Соколовы'})) {
 const runtime = {gateway: {subscription: vi.fn().mockResolvedValue({name:'Моя семья',activeChildren:0,childLimit:3,paid:false,access:false,accessUntil:'2026-10-29T10:00:00Z'}),renameFamily}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><FamilySubscription /></RuntimeContext.Provider>)
 return renameFamily
}
it('lets a parent save the family name even without an active subscription', async () => {
 const renameFamily = setup()
 fireEvent.change(await screen.findByLabelText('Название семьи'), {target:{value:'  Соколовы  '}})
 fireEvent.click(screen.getByRole('button',{name:'Сохранить название'}))
 expect((await screen.findByRole('status')).textContent).toBe('Название семьи сохранено.')
 expect(renameFamily).toHaveBeenCalledWith('Соколовы')
 expect((screen.getByLabelText('Название семьи') as HTMLInputElement).value).toBe('Соколовы')
 expect((screen.getByRole('button',{name:'Сохранить название'}) as HTMLButtonElement).disabled).toBe(true)
})
it('retains unsaved text when saving fails and permits retry', async () => {
 const renameFamily = setup(vi.fn().mockRejectedValueOnce(new Error('Нет связи')).mockResolvedValueOnce({name:'Соколовы'}))
 fireEvent.change(await screen.findByLabelText('Название семьи'), {target:{value:'Соколовы'}})
 fireEvent.click(screen.getByRole('button',{name:'Сохранить название'}))
 expect((await screen.findByRole('alert')).textContent).toBe('Нет связи')
 expect((screen.getByLabelText('Название семьи') as HTMLInputElement).value).toBe('Соколовы')
 fireEvent.click(screen.getByRole('button',{name:'Сохранить название'}))
 await waitFor(()=>expect(renameFamily).toHaveBeenCalledTimes(2))
 await screen.findByRole('status')
})
