import type { Participant } from './models'

export type AccountEntry = Participant[] | { onboarding: true }
export type FamilyRegistration = { name: string; parentName: string; pin: string }
