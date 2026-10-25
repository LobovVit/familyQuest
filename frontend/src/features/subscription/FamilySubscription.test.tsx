// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FamilySubscription } from './FamilySubscription'
const { subscription } = vi.hoisted(() => ({ subscription: vi.fn() }))
vi.mock('../../application/runtime', () => ({ useRuntime: () => ({ gateway: { subscription } }) }))
afterEach(cleanup)
describe('FamilyQuest subscription', () => {
 it('preserves permanent access without inviting another payment', async () => {
  subscription.mockResolvedValue({ name:'Existing family',activeChildren:1,childLimit:3,accessUntil:null,paid:false,access:true })
  render(<FamilySubscription />)
  expect(await screen.findByText('Срок доступа не ограничен')).toBeTruthy()
  expect(screen.getByText(/Оформлять новую платную подписку не нужно/)).toBeTruthy()
  expect(screen.queryByRole('link')).toBeNull()
 })
 it('shows unlimited children and the central cabinet for managed access', async () => {
  subscription.mockResolvedValue({ name:'Managed family',activeChildren:8,childLimit:3,childrenUnlimited:true,billingManaged:true,accessUntil:'2026-12-01T00:00:00Z',paid:true,access:true })
  render(<FamilySubscription />)
  expect(await screen.findByText(/8 · без ограничения количества/)).toBeTruthy()
  expect(screen.getByRole('link',{name:/Управлять подпиской/}).getAttribute('href')).toBe('https://lovit.tech/account')
 })
})
