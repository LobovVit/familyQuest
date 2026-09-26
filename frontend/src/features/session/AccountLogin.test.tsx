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
