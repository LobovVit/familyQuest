"use strict";
(() => {
  if (window.DelodenAssistant) return;
  const script = document.currentScript;
  const names = {"":"Делодень",familyquest:"Семейные дела","lovit-ritm":"Тренер","lovit-lessons":"Занятия","lovit-soglasovano":"Согласовано","lovit-master":"Заказы","lovit-menu":"Меню недели","lovit-dom":"Дом"};
  let active;
  function mount({product="", portalOrigin="https://lovit.tech"}={}) {
    if (!Object.hasOwn(names,product)) return {destroy(){}};
    if (active) return active;
    let portal;
    try { portal=new URL(portalOrigin); } catch { return {destroy(){}}; }
    const local=["localhost","127.0.0.1","[::1]"].includes(portal.hostname);
    if (portal.origin!=="https://lovit.tech" && !(local && ["http:","https:"].includes(portal.protocol))) return {destroy(){}};
    const launcher=document.createElement("button");
    launcher.id="deloden-assistant-launcher"; launcher.type="button";
    launcher.setAttribute("aria-label","Открыть помощника Делодня"); launcher.setAttribute("aria-haspopup","dialog");launcher.setAttribute("aria-expanded","false");launcher.setAttribute("aria-controls","deloden-assistant-dialog");
    // Original code-drawn paperclip. This constant markup never contains model/user text.
    launcher.innerHTML='<svg viewBox="0 0 64 74" aria-hidden="true"><path d="M21 49V24c0-12 25-15 25 1v28c0 21-35 23-35 0V24C11 1 56 1 56 26v25" fill="none" stroke="#465b41" stroke-width="5" stroke-linecap="round"/><path d="M22 45V25c0-9 13-9 13 0v29" fill="none" stroke="#829374" stroke-width="4" stroke-linecap="round"/><ellipse cx="29" cy="24" rx="6" ry="8" fill="white" stroke="#465b41" stroke-width="2"/><ellipse cx="43" cy="24" rx="6" ry="8" fill="white" stroke="#465b41" stroke-width="2"/><circle cx="30" cy="26" r="2.5" fill="#263626"/><circle cx="42" cy="26" r="2.5" fill="#263626"/></svg><span>Помощник</span>';
    const dialog=document.createElement("dialog");dialog.id="deloden-assistant-dialog";dialog.setAttribute("aria-labelledby","deloden-assistant-title");
    const header=document.createElement("header");header.className="deloden-assistant-heading";
    const title=document.createElement("strong");title.id="deloden-assistant-title";title.textContent="Скрепка · "+names[product];
    const separate=document.createElement("a");separate.textContent="↗";separate.title="Открыть помощника отдельно";separate.setAttribute("aria-label",separate.title);separate.target="_blank";separate.rel="noopener noreferrer";
    const standalone=new URL("/assistant",portal.origin);if(product)standalone.searchParams.set("product",product);separate.href=standalone.href;
    const close=document.createElement("button");close.type="button";close.textContent="×";close.setAttribute("aria-label","Свернуть помощника");
    const headingActions=document.createElement("div");headingActions.append(separate,close);header.append(title,headingActions);
    const frame=document.createElement("iframe");frame.title="Помощник: "+names[product];frame.referrerPolicy="origin";frame.setAttribute("sandbox","allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox");
    frame.height="520";frame.style.height="520px";
    dialog.append(header,frame);document.body.append(launcher,dialog);
    let loaded=false;
    function open() {
      if (!loaded) {
        const url=new URL(standalone);url.searchParams.set("embed","1");url.searchParams.set("parentOrigin",location.origin);
        frame.src=url.href;loaded=true;
      }
      if (!dialog.open) dialog.showModal();
      launcher.setAttribute("aria-expanded","true");close.focus();
    }
    function resize(height) {
      const maximum=Math.max(200,Math.min(740,window.innerHeight-110));
      frame.height=String(Math.round(Math.max(200,Math.min(maximum,height))));frame.style.height=frame.height+"px";
    }
    function receive(event) {
      const data=event.data;
      if (event.origin!==portal.origin || event.source!==frame.contentWindow || !data || data.product!==product) return;
      if (data.type==="deloden:assistant-close") {if(dialog.open)dialog.close();return}
      if (data.type!=="deloden:assistant-size" || !Number.isFinite(data.height)) return;
      resize(data.height);
    }
    function intercept(event) {
      if (event.defaultPrevented || event.button!==0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const link=event.target.closest?.("a[href]");if(!link)return;
      let url;try{url=new URL(link.href,location.href)}catch{return}
      if(url.origin===portal.origin && url.pathname==="/assistant" && (url.searchParams.get("product")||"")===product){event.preventDefault();open()}
    }
    const viewport=()=>resize(Number(frame.height));
    const refresh=()=>{if(loaded)frame.contentWindow.postMessage({type:"deloden:assistant-focus",product},portal.origin)};
    launcher.addEventListener("click",open);close.addEventListener("click",()=>dialog.close());
    dialog.addEventListener("close",()=>{launcher.setAttribute("aria-expanded","false");launcher.focus()});
    window.addEventListener("message",receive);window.addEventListener("resize",viewport);window.addEventListener("focus",refresh);document.addEventListener("click",intercept);
    active={open,destroy(){window.removeEventListener("message",receive);window.removeEventListener("resize",viewport);window.removeEventListener("focus",refresh);document.removeEventListener("click",intercept);dialog.remove();launcher.remove();active=null}};
    return active;
  }
  window.DelodenAssistant={mount};
  window.dispatchEvent(new Event("deloden-assistant-ready"));
  if (script && script.dataset.manual!=="true") mount({product:script.dataset.product||"",portalOrigin:script.dataset.portal||"https://lovit.tech"});
})();
