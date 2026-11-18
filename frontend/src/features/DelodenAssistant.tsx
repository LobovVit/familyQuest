import { useEffect } from 'react'

declare global {
  interface Window {
    DelodenAssistant?: {
      mount(options: { product: string }): { destroy(): void }
    }
  }
}

// Changing from a parent to a child destroys the iframe and its in-memory draft.
export function DelodenAssistant({ enabled }: { enabled: boolean }) {
  useEffect(() => {
    if (!enabled) return
    let mounted: { destroy(): void } | undefined
    const mount = () => {
      if (!mounted) mounted = window.DelodenAssistant?.mount({ product: 'familyquest' })
    }
    window.addEventListener('deloden-assistant-ready', mount)
    mount()
    return () => {
      window.removeEventListener('deloden-assistant-ready', mount)
      mounted?.destroy()
    }
  }, [enabled])
  return null
}
