// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { AccountLogin } from './AccountLogin'
import { loginNavigation } from './loginNavigation'
afterEach(cleanup)
it('uses the adult account and clears the password after login',async()=>{
 const response={participant:{id:1,familyId:2,name:'Родитель',role:'parent',active:true},token:'test'}
 const accountLogin=vi.fn().mockResolvedValue(response),saveSession=vi.fn()
 const runtime={gateway:{accountLogin},session:{subscribe:()=>()=>{},getParticipant:()=>null,clearSession:vi.fn(),saveSession}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin/></RuntimeContext.Provider>)
 fireEvent.change(screen.getByLabelText('Почта взрослого'),{target:{value:'parent@example.test'}})
 fireEvent.change(screen.getByLabelText('Пароль'),{target:{value:'a-long-password'}})
 fireEvent.click(screen.getByRole('button',{name:'Войти'}))
 await waitFor(()=>expect(saveSession).toHaveBeenCalledWith(response))
 expect(accountLogin).toHaveBeenCalledWith('parent@example.test','a-long-password')
 expect((screen.getByLabelText('Пароль') as HTMLInputElement).value).toBe('')
})
it('submits actual input values when autofill does not emit change events',async()=>{
 const response={participant:{id:1,familyId:2,name:'Родитель',role:'parent',active:true},token:'test'}
 const accountLogin=vi.fn().mockResolvedValue(response),saveSession=vi.fn()
 const runtime={gateway:{accountLogin},session:{subscribe:()=>()=>{},getParticipant:()=>null,clearSession:vi.fn(),saveSession}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin/></RuntimeContext.Provider>)
 ;(screen.getByLabelText('Почта взрослого') as HTMLInputElement).value='parent@example.test'
 ;(screen.getByLabelText('Пароль') as HTMLInputElement).value='filled-by-browser'
 fireEvent.click(screen.getByRole('button',{name:'Войти'}))
 await waitFor(()=>expect(accountLogin).toHaveBeenCalledWith('parent@example.test','filled-by-browser'))
})
it('uses centralized login without an application password form in SSO mode', async () => {
 const runtime={gateway:{restoreAccount:vi.fn().mockResolvedValue(null)},session:{subscribe:()=>()=>{},getParticipant:()=>null}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso /></RuntimeContext.Provider>)
 await waitFor(()=>expect(screen.getByRole('link',{name:'Войти с единым аккаунтом'}).getAttribute('href')).toBe('/api/account/authorize'))
 expect(screen.queryByLabelText('Пароль')).toBeNull()
})
it('completes SSO before opening the family workspace', async () => {
 window.history.replaceState(null,'','/?sso=complete')
 const response={participant:{id:1,familyId:2,name:'Родитель',role:'parent',active:true},token:'test'}
 const ssoSession=vi.fn().mockResolvedValue([response.participant]),saveSession=vi.fn(),selectAccountProfile=vi.fn().mockResolvedValue(response)
 const runtime={gateway:{ssoSession,selectAccountProfile},session:{subscribe:()=>()=>{},getParticipant:()=>null,clearSession:vi.fn(),saveSession}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso /></RuntimeContext.Provider>)
 await screen.findByLabelText('Семейный профиль')
 expect(saveSession).not.toHaveBeenCalled()
 fireEvent.change(screen.getByLabelText('Семейный профиль'),{target:{value:'1'}})
 fireEvent.change(screen.getByLabelText('PIN профиля'),{target:{value:'123456'}})
 fireEvent.click(screen.getByRole('button',{name:'Открыть профиль'}))
 await waitFor(()=>expect(saveSession).toHaveBeenCalledWith(response))
 expect(selectAccountProfile).toHaveBeenCalledWith(1,'123456')
 expect(window.location.search).toBe('')
})

it('redirects to LOVIT only after discovering that no adult session exists', async () => {
 window.history.replaceState(null,'','/')
 const open=vi.spyOn(loginNavigation,'open').mockImplementation(()=>{})
 const restoreAccount=vi.fn().mockResolvedValue(null)
 const runtime={gateway:{restoreAccount},session:{subscribe:()=>()=>{},getParticipant:()=>null}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso loginUrl="https://lovit.tech/login?product=familyquest" /></RuntimeContext.Provider>)
 await waitFor(()=>expect(open).toHaveBeenCalledWith('https://lovit.tech/login?product=familyquest'))
 expect(screen.queryByText('Запомнить это устройство на 30 дней')).toBeNull()
 open.mockRestore()
})
it('keeps a remembered adult in the profile chooser and still requires a PIN', async () => {
 window.history.replaceState(null,'','/')
 const open=vi.spyOn(loginNavigation,'open').mockImplementation(()=>{})
 const runtime={gateway:{restoreAccount:vi.fn().mockResolvedValue([{id:1,active:true,name:'Родитель'}])},session:{subscribe:()=>()=>{},getParticipant:()=>null}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso loginUrl="https://lovit.tech/login?product=familyquest" /></RuntimeContext.Provider>)
 await screen.findByLabelText('PIN профиля')
 expect(open).not.toHaveBeenCalled()
 expect((screen.getByRole('button',{name:'Открыть профиль'}) as HTMLButtonElement).disabled).toBe(true)
 open.mockRestore()
})
it.each([false,true])('does not loop on a failed restore or callback (callback=%s)', async complete => {
 window.history.replaceState(null,'',complete?'/?sso=complete':'/')
 const open=vi.spyOn(loginNavigation,'open').mockImplementation(()=>{})
 const fail=vi.fn().mockRejectedValue(new Error('Вход временно недоступен'))
 const runtime={gateway:{restoreAccount:fail,ssoSession:fail},session:{subscribe:()=>()=>{},getParticipant:()=>null}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso loginUrl="https://lovit.tech/login?product=familyquest" /></RuntimeContext.Provider>)
 expect((await screen.findByRole('alert')).textContent).toBe('Вход временно недоступен')
 expect(open).not.toHaveBeenCalled()
 expect(screen.getByRole('link',{name:'Продолжить в Делодень'}).getAttribute('href')).toBe('https://lovit.tech/login?product=familyquest')
 open.mockRestore()
 window.history.replaceState(null,'','/')
})
