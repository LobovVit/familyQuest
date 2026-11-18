// @vitest-environment jsdom
import { cleanup, render } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { DelodenAssistant } from './DelodenAssistant'

afterEach(() => { cleanup(); delete window.DelodenAssistant })

it('does not mount for a child; destroys the parent iframe when switching profiles', () => {
  const destroy = vi.fn(), mount = vi.fn(() => ({ destroy }))
  window.DelodenAssistant = { mount }
  const view = render(<DelodenAssistant enabled={false} />)
  expect(mount).not.toHaveBeenCalled()
  view.rerender(<DelodenAssistant enabled />)
  expect(mount).toHaveBeenCalledExactlyOnceWith({ product: 'familyquest' })
  view.rerender(<DelodenAssistant enabled={false} />)
  expect(destroy).toHaveBeenCalledTimes(1)
})

it('waits for the shared widget script and removes the listener on unmount', () => {
  const view = render(<DelodenAssistant enabled />)
  const destroy = vi.fn(), mount = vi.fn(() => ({ destroy }))
  window.DelodenAssistant = { mount }
  window.dispatchEvent(new Event('deloden-assistant-ready'))
  expect(mount).toHaveBeenCalledTimes(1)
  view.unmount()
  window.dispatchEvent(new Event('deloden-assistant-ready'))
  expect(mount).toHaveBeenCalledTimes(1)
  expect(destroy).toHaveBeenCalledTimes(1)
})
