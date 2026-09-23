import { useEffect, useRef, useState } from 'react'
import { useRuntime } from './runtime'

export function useParentConfirmation() {
 const { gateway } = useRuntime()
 const pending = useRef<((proof:string|null)=>void)|null>(null)
 const [open,setOpen]=useState(false),[pin,setPin]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('')
 useEffect(()=>()=>{pending.current?.(null);pending.current=null},[])
 function ask():Promise<string|null>{
  pending.current?.(null);setPin('');setError('');setOpen(true)
  return new Promise(resolve=>{pending.current=resolve})
 }
 function cancel(){if(busy)return;pending.current?.(null);pending.current=null;setOpen(false);setPin('')}
 async function submit(){
  if(busy||pin.length!==6)return
  setBusy(true);setError('')
  try{const {proof}=await gateway.confirmParent(pin);pending.current?.(proof);pending.current=null;setOpen(false);setPin('')}
  catch(e){setError(e instanceof Error?e.message:'Не удалось подтвердить PIN')}
  finally{setBusy(false)}
 }
 return {ask,open,pin,setPin,busy,error,cancel,submit}
}
