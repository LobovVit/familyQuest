import type { LoginOptions, TrustedDevice } from '../domain/devices'
import type { MathSettings, MathView, ActivityReward } from '../domain/learning'
import type { FamilyOverview, FamilyAction, FamilyDraft, FamilyEntry, FamilyKind } from '../domain/family'
import type { Assignment, BehaviorRating, Chore, ChoreDraft, LeaderboardEntry, LoginResponse, Participant, Reward, RewardPeriod, RewardType, Task } from '../domain/models'

export type ParticipantDraft = { name: string; role: Participant['role']; pin: string }
export type RewardDraft = { title: string; description: string; period: RewardPeriod; rewardType: RewardType; starCost: number; smileCost: number; participantIds: number[] }
export interface FamilyQuestGateway {
 finishMath(id: string): Promise<MathView>
 mathSessions(): Promise<MathView[]>
 startMath(settings: MathSettings): Promise<MathView>
 answerMath(id: string, index: number, values: Record<string, number>): Promise<MathView>
 activityRewards(): Promise<ActivityReward[]>

 familyOverview(date: string): Promise<FamilyOverview>
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
 restoreSession(): Promise<LoginResponse | null>
 logout(): Promise<unknown>
 confirmParent(pin: string): Promise<{proof: string}>
 devices(): Promise<TrustedDevice[]>
 revokeDevice(id: string, proof: string): Promise<unknown>
 login(id: number, pin: string, options?: LoginOptions): Promise<LoginResponse>
 saveChore(id: number | 'new', draft: ChoreDraft): Promise<unknown>
 completeTask(id: number): Promise<unknown>
 confirmTask(id: number, rating: number): Promise<unknown>
 rateBehavior(date: string, target: number, rating: number): Promise<BehaviorRating>
 createParticipant(input: ParticipantDraft): Promise<unknown>
 deleteParticipant(id: number): Promise<unknown>
 changePin(id: number, pin: string, proof?: string): Promise<unknown>
 createReward(input: RewardDraft): Promise<unknown>
 deleteReward(id: number): Promise<unknown>
}
export interface SessionStore {
 getParticipant(): Participant | null
 saveSession(session: LoginResponse): void
 clearSession(broadcast?: boolean): void
 subscribe(listener: () => void): () => void
}
export interface Runtime {
 gateway: FamilyQuestGateway
 session: SessionStore
 downloadBackup(date: string): Promise<void>
 restoreBackup(file: File, proof?: string): Promise<void>
}
