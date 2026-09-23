import type { useParentConfirmation } from '../../application/useParentConfirmation'
export function ParentConfirmation({confirmation:c}:{confirmation:ReturnType<typeof useParentConfirmation>}){
 if(!c.open)return null
 return <div className="pin-backdrop"><form className="pin-dialog" role="dialog" aria-modal="true" aria-label="Подтверждение родителя" onSubmit={e=>{e.preventDefault();void c.submit()}}><h2>Подтвердите действие</h2><p>Введите PIN своего родительского профиля.</p><label>PIN родителя<input autoFocus type="password" inputMode="numeric" autoComplete="off" maxLength={6} value={c.pin} onChange={e=>c.setPin(e.target.value.replace(/\D/g,'').slice(0,6))}/></label>{c.error&&<p role="alert">{c.error}</p>}<div className="pin-actions"><button type="button" disabled={c.busy} onClick={c.cancel}>Отмена</button><button disabled={c.busy||c.pin.length!==6}>Подтвердить</button></div></form></div>
}
