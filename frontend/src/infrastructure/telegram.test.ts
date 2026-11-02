// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest'
import { createTelegramGateway, createTelegramHost, isTelegramPage } from './telegram'
afterEach(()=>{vi.unstubAllGlobals();delete window.Telegram;window.history.replaceState(null,'','/');document.documentElement.removeAttribute('style')})
it('exchanges initData once in the POST body, keeps Portal tokens out of browser storage',async()=>{
 window.history.replaceState(null,'','/telegram')
 window.Telegram={WebApp:{initData:'signed-test',ready:vi.fn(),expand:vi.fn(),onEvent:vi.fn(),offEvent:vi.fn()}}
 const fetcher=vi.fn().mockResolvedValue(new Response(JSON.stringify({linked:false,families:[],accountUrl:'https://portal.example/telegram'})))
 vi.stubGlobal('fetch',fetcher)
 const client=createTelegramGateway();await Promise.all([client.start(),client.start()])
 expect(fetcher).toHaveBeenCalledTimes(1)
 const [path,init]=fetcher.mock.calls[0];expect(path).toBe('/api/telegram/exchange')
 expect(JSON.parse(init.body)).toEqual({initData:'signed-test'})
 expect(init.headers.get('X-FamilyQuest-Client')).toBe('telegram')
 expect(JSON.stringify(sessionStorage)).not.toContain('signed-test')
})
it('follows Telegram theme, safe area, viewport and BackButton; removes handlers',()=>{
 const handlers=new Map<string,()=>void>(),show=vi.fn(),hide=vi.fn(),onClick=vi.fn(),offClick=vi.fn()
 const app={initData:'signed-test',colorScheme:'dark',themeParams:{bg_color:'#102030'},viewportStableHeight:650,safeAreaInset:{top:20},contentSafeAreaInset:{top:12},ready:vi.fn(),expand:vi.fn(),onEvent:(event:string,fn:()=>void)=>handlers.set(event,fn),offEvent:(event:string)=>handlers.delete(event),BackButton:{show,hide,onClick,offClick}}
 window.Telegram={WebApp:app};const host=createTelegramHost();const dispose=host.mount()
 expect(document.documentElement.dataset.telegram).toBe('dark');expect(document.documentElement.style.getPropertyValue('--mini-top')).toBe('32px')
 expect(document.documentElement.style.getPropertyValue('--mini-height')).toBe('650px')
 app.colorScheme='light';handlers.get('themeChanged')?.();expect(document.documentElement.dataset.telegram).toBe('light')
 const back=vi.fn(),stop=host.back(back);expect(onClick).toHaveBeenCalledWith(back);stop();expect(offClick).toHaveBeenCalledWith(back)
 dispose();expect(handlers.size).toBe(0);expect(document.documentElement.dataset.telegram).toBeUndefined()
})
it('leaves the ordinary web route outside the Telegram shell',()=>{expect(isTelegramPage()).toBe(false);expect(createTelegramHost().available()).toBe(false)})
