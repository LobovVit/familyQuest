// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { RuntimeContext } from '../../application/runtime'
import type { Runtime } from '../../application/ports'
import { LearningProfiles } from './LearningProfiles'
afterEach(cleanup)
it('saves the birth date entered by a native date input', async () => {
 const saveLearningProfile = vi.fn().mockResolvedValue(undefined)
 const runtime = {gateway: {learningProfile: vi.fn().mockResolvedValue({birthDate:'',mathLevel:'',readingLevel:''}),saveLearningProfile}} as unknown as Runtime
 render(<RuntimeContext.Provider value={runtime}><LearningProfiles participants={[{id:2,name:'Ребёнок',role:'child',active:true}]}/></RuntimeContext.Provider>)
 const input = await screen.findByLabelText('Дата рождения')
 fireEvent.input(input,{target:{value:'2018-09-26'}})
 fireEvent.click(screen.getByRole('button',{name:'Сохранить настройки'}))
 await waitFor(()=>expect(saveLearningProfile).toHaveBeenCalledWith(2,{birthDate:'2018-09-26',mathLevel:'',readingLevel:''}))
 expect((await screen.findByRole('status')).textContent).toContain('Сохранено')
})
