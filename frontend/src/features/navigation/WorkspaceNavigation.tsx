import type { NavigationItem, WorkspaceSection } from './sections'
export function WorkspaceNavigation({ items, active, onSelect }: { items: NavigationItem[]; active: WorkspaceSection; onSelect: (id: WorkspaceSection) => void }) {
 return <nav className="workspace-navigation" aria-label="Разделы FamilyQuest">
  <div className="navigation-brand"><span aria-hidden="true">✦</span> FamilyQuest<small>Каждый день — вместе</small></div>
  <div className="navigation-items">{items.map(item => <button key={item.id} type="button" aria-current={active === item.id ? 'page' : undefined} className={active === item.id ? 'navigation-item is-current' : 'navigation-item'} onClick={() => onSelect(item.id)}><span aria-hidden="true">{item.icon}</span><span>{item.label}</span></button>)}</div>
 </nav>
}
