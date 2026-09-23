export type LoginOptions = { remember: boolean; deviceName: string; parentId?: number; parentPin?: string }
export type TrustedDevice = { id: string; participantId: number; participantName: string; name: string; createdAt: string; lastSeenAt: string; expiresAt: string; current: boolean }
