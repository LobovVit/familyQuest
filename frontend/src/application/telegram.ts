import type { LoginResponse, Participant } from '../domain/models'
export type TelegramState = { linked: boolean; families: {id:number;name:string}[]; accountUrl:string; expiresAt:string }
export interface TelegramGateway {
 start(): Promise<TelegramState>
 state(): Promise<TelegramState>
 claim(linkCode:string): Promise<unknown>
 family(familyId:number): Promise<Participant[] | TelegramState>
 profile(participantId:number,pin:string): Promise<LoginResponse>
 consent(enabled:boolean): Promise<unknown>
 logout(): Promise<unknown>
}
export interface TelegramHost {
 available(): boolean
 startParam(): string
 mount(): () => void
 back(action:(()=>void)|null): () => void
 requestWriteAccess(): Promise<boolean>
}
