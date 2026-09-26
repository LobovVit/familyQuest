import type { MathSettings } from './learning'
import type { ReadingMode } from './reading'
export type LearningProfile = { birthDate: string; mathLevel: '' | MathSettings['level']; readingLevel: '' | ReadingMode }
export type LearningPolicy = { age?: number; mathLevel: MathSettings['level']; readingLevel: ReadingMode; version: number }
export type FamilySubscription = { familyId: number; name: string; registeredAt: string; status: string; plan: string; childLimit: number; activeChildren: number; accessUntil: string | null; paid: boolean; access: boolean }
export function allowsMath(policy: LearningPolicy, level: MathSettings['level']): boolean {
 const ranks = { easy: 1, medium: 2, hard: 3, columnar: 3 }
 return level === 'columnar' ? policy.mathLevel === 'columnar' : ranks[level] <= ranks[policy.mathLevel]
}
export function allowsReading(policy: LearningPolicy, level: ReadingMode): boolean {
 const ranks = { phrases: 1, sentences: 2, advanced: 3 }
 return ranks[level] <= ranks[policy.readingLevel]
}
