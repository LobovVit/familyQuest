export type MCPGrant = {
 id: string
 participantId: number
 name: string
 readOnly: boolean
 createdAt: string
 expiresAt: string
}
export type MCPGrantInput = { participantId: number; name: string; readOnly: boolean }
