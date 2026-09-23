import { describe, expect, it, vi } from 'vitest'
import { createSessionLifecycle } from './sessionLifecycle'
import type { LoginResponse } from '../domain/models'
const login: LoginResponse = {participant:{id:1,name:'Parent',role:'parent',active:true},token:'temporary'}
function fixture() {
 let generation=0
 const session={getSessionGeneration:()=>generation,getParticipant:()=>null,subscribe:()=>()=>{},saveSession:vi.fn(()=>{generation++}),clearSession:vi.fn(()=>{generation++})}
 return {session}
}
describe('session lifecycle use case',()=>{
 it('shares startup and clears an invalid session',async()=>{
  const {session}=fixture();const gateway={restoreSession:vi.fn(async()=>null)}
  const lifecycle=createSessionLifecycle(gateway,session)
  await Promise.all([lifecycle.initialize(),lifecycle.initialize()])
  expect(gateway.restoreSession).toHaveBeenCalledTimes(1);expect(session.clearSession).toHaveBeenCalledTimes(1)
 })
 it('does not replace a newer login with a stale response',async()=>{
  const {session}=fixture();let resolve!:(v:LoginResponse)=>void
  const lifecycle=createSessionLifecycle({restoreSession:()=>new Promise(r=>{resolve=r})},session)
  const pending=lifecycle.refresh();session.clearSession();resolve(login);await pending
  expect(session.saveSession).not.toHaveBeenCalled()
 })
 it('keeps local identity on network failure and allows retry',async()=>{
  const {session}=fixture();const gateway={restoreSession:vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue(login)}
  const lifecycle=createSessionLifecycle(gateway,session)
  await expect(lifecycle.initialize()).rejects.toThrow('offline')
  expect(session.clearSession).not.toHaveBeenCalled()
  await lifecycle.initialize();expect(session.saveSession).toHaveBeenCalledWith(login,false)
 })
})
