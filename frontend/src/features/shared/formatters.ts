import type { Assignment, LeaderboardEntry, Task } from '../../domain/models'
export function statusLabel(status:Task['status']) { return status==='completed'?'Ждет оценки':status==='confirmed'?'Принято':status==='needs_work'?'Доработать':'Запланировано' }
export function taskRewardLabel(assignments:Assignment[],task:Task) { const base=assignments.find(a=>a.id===task.assignmentId)?.baseValue??0; return task.averageRating>0?`${task.reward.toFixed(0)} ⭐ · оценка ${task.averageRating.toFixed(1)}/5`:`${base} ⭐ за выполнение` }
export function formatDate(value:string) { const date=new Date(`${value}T00:00:00`);const months=['Января','Февраля','Марта','Апреля','Мая','Июня','Июля','Августа','Сентября','Октября','Ноября','Декабря'];return `${date.getDate()} ${months[date.getMonth()]} ${date.getFullYear()}` }
export function behaviorLabel(entry:LeaderboardEntry) { return entry.behaviorCount===0?'нет оценок':`${entry.behaviorRating.toFixed(1)}/5` }
