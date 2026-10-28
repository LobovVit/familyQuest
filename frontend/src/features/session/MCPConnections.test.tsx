// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { MCPConnections } from './MCPConnections'
afterEach(cleanup)
const participants = [{ id: 2, name: 'Ребёнок', role: 'child' as const, active: true, createdAt: '' }]
it('creates a delegated key only after parent confirmation, shows it once and revokes it', async () => {
 const grant = {id:'grant-1',participantId:2,name:'Помощник',readOnly:true,expiresAt:'2027-01-01',createdAt:'2026-10-19'}
 const gateway = {mcpGrants: vi.fn().mockResolvedValue([]),createMCPGrant:vi.fn().mockResolvedValue({grant,token:'secret-key'}),revokeMCPGrant:vi.fn().mockResolvedValue({})}
 const confirm = vi.fn().mockResolvedValue('parent-proof')
 render(<RuntimeContext.Provider value={{gateway} as unknown as Runtime}><MCPConnections participants={participants} confirm={confirm}/></RuntimeContext.Provider>)
 await screen.findByText('Пока нет подключений.')
 fireEvent.change(screen.getByLabelText('Профиль для ИИ'),{target:{value:'2'}})
 fireEvent.change(screen.getByLabelText('Название подключения'),{target:{value:'Помощник'}})
 expect((screen.getByLabelText('Только чтение результатов') as HTMLInputElement).checked).toBe(true)
 fireEvent.click(screen.getByRole('button',{name:'Создать ключ MCP'}))
 await screen.findByLabelText('Новый ключ MCP')
 expect(confirm).toHaveBeenCalledOnce()
 expect(gateway.createMCPGrant).toHaveBeenCalledWith({participantId:2,name:'Помощник',readOnly:true},'parent-proof')
 fireEvent.click(screen.getByRole('button',{name:'Скрыть ключ'}))
 expect(screen.queryByDisplayValue('secret-key')).toBeNull()
 fireEvent.click(screen.getByRole('button',{name:'Отключить Помощник'}))
 await waitFor(()=>expect(gateway.revokeMCPGrant).toHaveBeenCalledWith('grant-1'))
 await screen.findByText('Пока нет подключений.')
})
it('does not issue a key when parent cancels confirmation', async () => {
 const gateway = {mcpGrants:vi.fn().mockResolvedValue([]),createMCPGrant:vi.fn()}
 render(<RuntimeContext.Provider value={{gateway} as unknown as Runtime}><MCPConnections participants={participants} confirm={async()=>null}/></RuntimeContext.Provider>)
 await screen.findByText('Пока нет подключений.')
 fireEvent.change(screen.getByLabelText('Профиль для ИИ'),{target:{value:'2'}})
 fireEvent.change(screen.getByLabelText('Название подключения'),{target:{value:'Помощник'}})
 fireEvent.click(screen.getByRole('button',{name:'Создать ключ MCP'}))
 await waitFor(()=>expect((screen.getByRole('button',{name:'Создать ключ MCP'}) as HTMLButtonElement).disabled).toBe(false))
 expect(gateway.createMCPGrant).not.toHaveBeenCalled()
})
