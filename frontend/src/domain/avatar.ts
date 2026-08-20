import type { Participant, ParticipantRole } from './models'

const roleAvatars: Record<ParticipantRole, string> = { parent: '🧑', child: '🧒', school: '🎒' }
export const participantAvatar = (participant: Participant): string => roleAvatars[participant.role]
export const avatarForRole = (role?: ParticipantRole): string => role ? roleAvatars[role] : '🙂'
