import { api } from './apiClient'
import type { TelegramGateway, TelegramHost } from '../application/telegram'

type Insets = {top?:number;bottom?:number;left?:number;right?:number}
type WebApp = {
 initData:string;initDataUnsafe?:{start_param?:string};colorScheme?:string;themeParams?:Record<string,string>;
 viewportStableHeight?:number;safeAreaInset?:Insets;contentSafeAreaInset?:Insets;
 ready():void;expand():void;onEvent(event:string,fn:()=>void):void;offEvent(event:string,fn:()=>void):void;
 BackButton?:{show():void;hide():void;onClick(fn:()=>void):void;offClick(fn:()=>void):void};
 requestWriteAccess?:(callback:(granted:boolean)=>void)=>void
}
declare global {interface Window {Telegram?:{WebApp?:WebApp}}}
export const isTelegramPage = () => window.location.pathname === '/telegram'
export async function loadTelegramSDK():Promise<void>{
 if(window.Telegram?.WebApp)return
 await new Promise<void>((resolve,reject)=>{
  const script=document.createElement('script');script.src='https://telegram.org/js/telegram-web-app.js';script.async=true
  const timer=window.setTimeout(()=>reject(new Error('Telegram SDK unavailable')),10000)
  script.onload=()=>{clearTimeout(timer);resolve()};script.onerror=()=>{clearTimeout(timer);reject(new Error('Telegram SDK unavailable'))};document.head.append(script)
 })
}
export function createTelegramHost():TelegramHost {
 const app=()=>window.Telegram?.WebApp
 return {
  available:()=>!!app()?.initData,
  startParam:()=>app()?.initDataUnsafe?.start_param ?? new URLSearchParams(window.location.search).get('tgWebAppStartParam') ?? '',
  mount:()=>{
   const tg=app();if(!tg)return ()=>{}
   const root=document.documentElement
   const update=()=>{
    root.dataset.telegram=tg.colorScheme==='dark'?'dark':'light'
    const colors:Record<string,string>={bg_color:'bg',text_color:'text',hint_color:'hint',button_color:'button',button_text_color:'button-text',secondary_bg_color:'panel'}
    for(const [key,name] of Object.entries(colors)){const value=tg.themeParams?.[key];if(value&&/^#[0-9a-f]{6}$/i.test(value))root.style.setProperty(`--mini-${name}`,value)}
    if(tg.viewportStableHeight&&Number.isFinite(tg.viewportStableHeight))root.style.setProperty('--mini-height',`${Math.max(0,tg.viewportStableHeight)}px`)
    for(const side of ['top','bottom','left','right'] as const){const inset=Math.max(0,tg.safeAreaInset?.[side]??0)+(Math.max(0,tg.contentSafeAreaInset?.[side]??0));root.style.setProperty(`--mini-${side}`,`${inset}px`)}
   }
   const events=['themeChanged','viewportChanged','safeAreaChanged','contentSafeAreaChanged'];update();events.forEach(event=>tg.onEvent(event,update));tg.ready();tg.expand()
   return ()=>{events.forEach(event=>tg.offEvent(event,update));delete root.dataset.telegram;for(const key of ['bg','text','hint','button','button-text','panel','height','top','bottom','left','right'])root.style.removeProperty(`--mini-${key}`)}
  },
  back:action=>{const back=app()?.BackButton;if(!back)return ()=>{};if(!action){back.hide();return ()=>{}};back.show();back.onClick(action);return ()=>{back.offClick(action);back.hide()}},
  requestWriteAccess:()=>new Promise(resolve=>{const tg=app();if(!tg?.requestWriteAccess){resolve(false);return};tg.requestWriteAccess(resolve)}),
 }
}
const post=(body:unknown)=>({method:'POST',body:JSON.stringify(body)})
export function createTelegramGateway():TelegramGateway {
 let startup:ReturnType<TelegramGateway['start']>|undefined
 return {
  // StrictMode/remounts must not replay the one-use Telegram exchange.
  start:()=>startup??=api('/api/telegram/exchange',post({initData:window.Telegram?.WebApp?.initData??''})),
  state:()=>api('/api/telegram/session',post({})),
  claim:linkCode=>api('/api/telegram/link',post({linkCode})),
  family:familyId=>api('/api/telegram/family',post({familyId})),
  profile:(participantId,pin)=>api('/api/telegram/profile',post({participantId,pin})),
  consent:enabled=>api('/api/telegram/consent',post({enabled})),
  logout:()=>api('/api/telegram/logout',post({})),
 }
}
