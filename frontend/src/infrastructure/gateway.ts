import type { FamilyQuestGateway } from '../application/ports'
import { api } from './apiClient'

const post = (body: unknown) => ({ method: 'POST', body: JSON.stringify(body) })
export const gateway: FamilyQuestGateway = {
 finishMath: id => api(`/api/math/${encodeURIComponent(id)}/finish`, post({})),
 mathSessions: () => api('/api/math'),
 startMath: settings => api('/api/math', post(settings)),
 answerMath: (id, index, values) => api(`/api/math/${encodeURIComponent(id)}/answers`, post({ index, values })),
 activityRewards: () => api('/api/activity-rewards'),

 familyOverview: date => api(`/api/family/overview?date=${encodeURIComponent(date)}`),
 familyEntries: () => api('/api/family'),
 createFamily: (kind, draft) => api('/api/family', post({ kind, draft })),
 editFamily: (id, version, draft) => api(`/api/family/${id}`, { ...post({ version, draft }), method: 'PUT' }),
 familyAction: (id, version, action) => api(`/api/family/${id}/actions`, post({ version, ...action })),
 participants: () => api('/api/participants'),
 chores: () => api('/api/chores'),
 assignments: () => api('/api/assignments'),
 rewards: () => api('/api/rewards'),
 tasks: date => api(`/api/tasks?date=${date}`),
 leaderboard: (period, date) => api(`/api/leaderboard?period=${period}&date=${date}`),
 ratings: date => api(`/api/behavior-ratings?date=${date}`),
 restoreSession: () => api('/api/session'),
 logout: () => api('/api/session/logout', post({})),
 confirmParent: pin => api('/api/session/confirm', post({pin})),
 devices: () => api('/api/devices'),
 revokeDevice: (id, proof) => api(`/api/devices/${id}`, {method:'DELETE',headers:{'X-FamilyQuest-Confirmation':proof}}),
 login: (participantId, pin, options) => api('/api/session', post({ participantId, pin, ...options })),
 saveChore: (id, draft) => api(id === 'new' ? '/api/chores' : `/api/chores/${id}`, { ...post(draft), method: id === 'new' ? 'POST' : 'PUT' }),
 completeTask: id => api(`/api/tasks/${id}/complete`, post({})),
 confirmTask: (id, rating) => api(`/api/tasks/${id}/confirm`, post({ rating })),
 rateBehavior: (date, targetParticipantId, rating) => api('/api/behavior-ratings', post({ date, targetParticipantId, rating })),
 createParticipant: input => api('/api/participants', post(input)),
 deleteParticipant: id => api(`/api/participants/${id}`, { method: 'DELETE' }),
 changePin: (id, pin, proof) => api(`/api/participants/${id}/pin`, { ...post({ pin }), method: 'PUT',headers:{'X-FamilyQuest-Confirmation':proof ?? ''} }),
 createReward: input => api('/api/rewards', post(input)),
 deleteReward: id => api(`/api/rewards/${id}`, { method: 'DELETE' }),
}
