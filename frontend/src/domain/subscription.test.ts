import { expect, it } from 'vitest'
import { allowsMath, allowsReading } from './subscription'
it('separates subject limits and column work from ordinary difficulty',()=>{
 const policy={mathLevel:'medium',readingLevel:'phrases',version:1} as const
 expect(allowsMath(policy,'easy')).toBe(true)
 expect(allowsMath(policy,'hard')).toBe(false)
 expect(allowsMath({...policy,mathLevel:'hard'},'columnar')).toBe(false)
 expect(allowsReading(policy,'sentences')).toBe(false)
})
