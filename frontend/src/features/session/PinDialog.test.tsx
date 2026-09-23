// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { PinDialog } from './PinDialog'
import type { Participant } from '../../domain/models'
const child:Participant={id:2,name:'Ребёнок',role:'child',active:true}
const parent:Participant={id:1,name:'Родитель',role:'parent',active:true}
afterEach(cleanup)
describe('device login',()=>{
 it('requires parent approval only for a remembered child login',()=>{
  const props={participant:child,participants:[child,parent],pin:'123456',busy:false,error:'',onPin:vi.fn(),onCancel:vi.fn(),onSubmit:vi.fn(),onOptions:vi.fn()}
  const {rerender}=render(<PinDialog {...props} options={{remember:false,deviceName:'iPad'}}/> )
  expect((screen.getByRole('button',{name:'Войти'}) as HTMLButtonElement).disabled).toBe(false)
  rerender(<PinDialog {...props} options={{remember:true,deviceName:'iPad',parentId:1}}/> )
  expect((screen.getByRole('button',{name:'Войти'}) as HTMLButtonElement).disabled).toBe(true)
  fireEvent.change(screen.getByLabelText('PIN родителя'),{target:{value:'12abc3456'}})
  expect(props.onOptions).toHaveBeenCalledWith({remember:true,deviceName:'iPad',parentId:1,parentPin:'123456'})
  rerender(<PinDialog {...props} options={{remember:true,deviceName:'iPad',parentId:1,parentPin:'123456'}}/> )
  expect((screen.getByRole('button',{name:'Войти'}) as HTMLButtonElement).disabled).toBe(false)
 })
 it('shows login errors within the dialog',()=>{
  render(<PinDialog participant={parent} participants={[parent]} pin="" options={{remember:false,deviceName:''}} error="Неверный PIN" busy={false} onOptions={vi.fn()} onPin={vi.fn()} onCancel={vi.fn()} onSubmit={vi.fn()}/> )
  expect(screen.getByRole('alert').textContent).toBe('Неверный PIN')
 })
})
