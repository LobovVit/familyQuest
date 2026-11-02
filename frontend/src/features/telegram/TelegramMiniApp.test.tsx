// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import type { TelegramGateway, TelegramHost } from '../../application/telegram'
import { TelegramMiniApp } from './TelegramMiniApp'
import { telegramSection } from './navigation'
afterEach(cleanup)
const parent={id:7,familyId:12,name:'Взрослый',role:'parent',active:true} as const
function setup(linked=true){
 const value={linked,families:[{id:12,name:'Моя семья'}],accountUrl:'https://portal.example/telegram',expiresAt:'2099-01-01T00:00:00Z'}
 const client={start:vi.fn().mockResolvedValue(value),state:vi.fn().mockResolvedValue(value),family:vi.fn().mockResolvedValue([parent]),profile:vi.fn().mockResolvedValue({participant:parent,token:'profile-only'}),claim:vi.fn().mockResolvedValue({}),consent:vi.fn(),logout:vi.fn()} satisfies TelegramGateway
 const host={available:()=>true,startParam:()=>'',mount:()=>()=>{},back:vi.fn().mockReturnValue(()=>{}),requestWriteAccess:vi.fn()} satisfies TelegramHost
 const runtime={gateway:{},session:{subscribe:()=>()=>{},getParticipant:()=>null,clearSession:vi.fn(),saveSession:vi.fn()}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><TelegramMiniApp client={client} host={host}/></RuntimeContext.Provider>)
 return {client,host,runtime}
}
it('requires explicit family and profile PIN; a linked identity grants no parent session',async()=>{
 const {client,runtime}=setup()
 await screen.findByRole('button',{name:'Моя семья'})
 expect(client.family).not.toHaveBeenCalled();expect(runtime.session.saveSession).not.toHaveBeenCalled()
 fireEvent.click(screen.getByRole('button',{name:'Моя семья'}))
 await screen.findByLabelText('Профиль');expect(client.family).toHaveBeenCalledWith(12)
 expect((screen.getByRole('button',{name:'Открыть профиль'}) as HTMLButtonElement).disabled).toBe(true)
 fireEvent.change(screen.getByLabelText('Профиль'),{target:{value:'7'}})
 fireEvent.change(screen.getByLabelText('PIN'),{target:{value:'739281'}})
 fireEvent.click(screen.getByRole('button',{name:'Открыть профиль'}))
 await waitFor(()=>expect(client.profile).toHaveBeenCalledWith(7,'739281'))
 await waitFor(()=>expect(runtime.session.saveSession).toHaveBeenCalledWith({participant:parent,token:'profile-only'}))
})
it('does not auto-link after entering a code or create a family',async()=>{
 const {client}=setup(false)
 await screen.findByLabelText('Код привязки')
 fireEvent.change(screen.getByLabelText('Код привязки'),{target:{value:'tgl_test_code'}})
 fireEvent.click(screen.getByRole('button',{name:'Отправить код'}))
 await waitFor(()=>expect(client.claim).toHaveBeenCalledWith('tgl_test_code'))
 expect(client.family).not.toHaveBeenCalled();expect(client.profile).not.toHaveBeenCalled()
 expect(screen.queryByRole('button',{name:'Моя семья'})).toBeNull()
})
it('clears the profile selection when returning to families',async()=>{
 const {client,runtime}=setup()
 fireEvent.click(await screen.findByRole('button',{name:'Моя семья'}))
 await screen.findByLabelText('Профиль')
 client.family.mockResolvedValueOnce({linked:true,families:[{id:12,name:'Моя семья'}],accountUrl:'https://portal.example/telegram',expiresAt:''} as never)
 fireEvent.click(screen.getByRole('button',{name:'Назад к семьям'}))
 await waitFor(()=>expect(client.family).toHaveBeenLastCalledWith(0))
 await waitFor(()=>expect(screen.queryByLabelText('Профиль')).toBeNull())
 expect(runtime.session.clearSession).toHaveBeenCalled()
})
it('accepts only known navigation hints, never a family or arbitrary URL',()=>{
 expect(telegramSection('assign')).toBe('catalog')
 expect(telegramSection('review')).toBe('day')
 expect(telegramSection('family_99')).toBe('day')
 expect(telegramSection('constructor')).toBe('day')
 expect(telegramSection('https://attacker.example')).toBe('day')
})
