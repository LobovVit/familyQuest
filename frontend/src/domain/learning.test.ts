import { describe, expect, it } from 'vitest'
import { mathStars, type MathSettings } from './learning'
describe('math reward preview', () => {
 const base: MathSettings = {operation: '+', level: 'easy', answerMode: 'input', divisionMode: 'full', allowNegative: false}
 it('keeps beginner success valuable and distinguishes extra work', () => {
  expect(mathStars(base)).toBe(2)
  expect(mathStars({...base,operation:'*'})).toBe(3)
  expect(mathStars({...base,level:'medium',answerMode:'choice'})).toBe(2)
  expect(mathStars({...base,level:'hard',operation:':'})).toBe(5)
  expect(mathStars({...base,level:'columnar',operation:':',divisionMode:'result'})).toBe(4)
  expect(mathStars({...base,level:'columnar',operation:':',divisionMode:'steps'})).toBe(5)
  expect(mathStars({...base,level:'columnar',operation:':'})).toBe(6)
 })
})
