import type { LeaderboardEntry } from '../../domain/models'
import { avatarForRole } from '../../domain/avatar'
import { behaviorLabel } from './formatters'
export function Leaderboard({entries,title}:{entries:LeaderboardEntry[];title:string}) { return <section className="panel"><div className="section-heading compact"><h2>{title}</h2></div><ol className="leaderboard">{entries.map(entry=><li key={entry.participantId}><span className="leaderboard-avatar">{avatarForRole()}</span><span className="leaderboard-name">{entry.name}</span><strong>{entry.reward.toFixed(0)} ⭐ · {entry.behaviorSmiles} 🙂</strong><small>{entry.tasksDone}/{entry.tasksAssigned} дел · дела {entry.averageRating.toFixed(1)}/5 · поведение {behaviorLabel(entry)}</small></li>)}</ol></section> }
