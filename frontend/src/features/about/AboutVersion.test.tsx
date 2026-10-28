// @vitest-environment jsdom
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { AboutVersion } from './AboutVersion'
afterEach(cleanup)
it('shows deployed server identity and warns about an old browser build', async () => {
 const build = {version:'0.1.0',commit:'new-commit',builtAt:'2026-09-27T00:00:00Z'}
 const runtime = {gateway:{version:vi.fn().mockResolvedValue({server:{...build,startedAt:build.builtAt},web:{...build,commit:'old-commit'}})}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AboutVersion /></RuntimeContext.Provider>)
 expect(await screen.findByText('О приложении · 0.1.0')).toBeTruthy()
 expect((await screen.findByRole('status')).textContent).toContain('разных сборок')
})
it('does not invent a deployed version when the endpoint fails', async () => {
 const runtime = {gateway:{version:vi.fn().mockRejectedValue(new Error('offline'))}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><AboutVersion /></RuntimeContext.Provider>)
 expect(await screen.findByText(/Не удалось проверить версию сервера/)).toBeTruthy()
 expect(screen.queryByText(/О приложении ·/)).toBeNull()
})
