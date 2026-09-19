import type { FamilyAction, FamilyDraft, FamilyEntry, FamilyKind } from '../domain/family'
import type { Assignment, BehaviorRating, Chore, ChoreDraft, LeaderboardEntry, LoginResponse, Participant, Reward, RewardPeriod, RewardType, Task } from '../domain/models'

export type ParticipantDraft = { name: string; role: Participant['role']; pin: string }
export type RewardDraft = { title: string; description: string; period: RewardPeriod; rewardType: RewardType; starCost: number; smileCost: number; participantIds: number[] }
export interface FamilyQuestGateway {
 familyEntries(): Promise<FamilyEntry[]>
 createFamily(kind: FamilyKind, draft: FamilyDraft): Promise<FamilyEntry>
 editFamily(id: number, version: number, draft: FamilyDraft): Promise<FamilyEntry>
 familyAction(id: number, version: number, action: FamilyAction): Promise<FamilyEntry>
 participants(): Promise<Participant[]>
 chores(): Promise<Chore[]>
 assignments(): Promise<Assignment[]>
 rewards(): Promise<Reward[]>
 tasks(date: string): Promise<Task[]>
 leaderboard(period: RewardPeriod, date: string): Promise<LeaderboardEntry[]>
 ratings(date: string): Promise<BehaviorRating[]>
 login(id: number, pin: string): Promise<LoginResponse>
 saveChore(id: number | 'new', draft: ChoreDraft): Promise<unknown>
 completeTask(id: number): Promise<unknown>
 confirmTask(id: number, rating: number): Promise<unknown>
 rateBehavior(date: string, target: number, rating: number): Promise<BehaviorRating>
 createParticipant(input: ParticipantDraft): Promise<unknown>
 deleteParticipant(id: number): Promise<unknown>
 changePin(id: number, pin: string): Promise<unknown>
 createReward(input: RewardDraft): Promise<unknown>
 deleteReward(id: number): Promise<unknown>
}
export interface SessionStore {
 getParticipant(): Participant | null
 saveSession(session: LoginResponse): void
 clearSession(): void
 subscribe(listener: () => void): () => void
}
export interface Runtime {
 gateway: FamilyQuestGateway
 session: SessionStore
 downloadBackup(date: string): Promise<void>
 restoreBackup(file: File): Promise<void>
}
