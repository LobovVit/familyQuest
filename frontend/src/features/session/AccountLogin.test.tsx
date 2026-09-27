// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { AccountLogin } from './AccountLogin'
afterEach(cleanup)
it('uses the adult account and clears the password after login',async()=>{
 const response={participant:{id:1,familyId:2,name:'Родитель',role:'parent',active:true},token:'test'}
 const accountLogin=vi.fn().mockResolvedValue(response),saveSession=vi.fn()
 const runtime={gateway:{accountLogin},session:{subscribe:()=>()=>{},getParticipant:()=>null,saveSession}} as unknown as Runtime
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
 const runtime={gateway:{accountLogin},session:{subscribe:()=>()=>{},getParticipant:()=>null,saveSession}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin/></RuntimeContext.Provider>)
 ;(screen.getByLabelText('Почта взрослого') as HTMLInputElement).value='parent@example.test'
 ;(screen.getByLabelText('Пароль') as HTMLInputElement).value='filled-by-browser'
 fireEvent.click(screen.getByRole('button',{name:'Войти'}))
 await waitFor(()=>expect(accountLogin).toHaveBeenCalledWith('parent@example.test','filled-by-browser'))
})
it('uses centralized login without an application password form in SSO mode', () => {
 const runtime={gateway:{},session:{subscribe:()=>()=>{},getParticipant:()=>null}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso /></RuntimeContext.Provider>)
 expect(screen.getByRole('link',{name:'Войти с единым аккаунтом'}).getAttribute('href')).toBe('/api/account/authorize')
 expect(screen.queryByLabelText('Пароль')).toBeNull()
})
it('completes SSO before opening the family workspace', async () => {
 window.history.replaceState(null,'','/?sso=complete')
 const response={participant:{id:1,familyId:2,name:'Родитель',role:'parent',active:true},token:'test'}
 const ssoSession=vi.fn().mockResolvedValue(response),saveSession=vi.fn()
 const runtime={gateway:{ssoSession},session:{subscribe:()=>()=>{},getParticipant:()=>null,saveSession}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AccountLogin sso /></RuntimeContext.Provider>)
 await waitFor(()=>expect(saveSession).toHaveBeenCalledWith(response))
 expect(window.location.search).toBe('')
})
