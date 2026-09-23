export type WorkspaceSection = 'today' | 'day' | 'math' | 'earned' | 'sport' | 'family' | 'catalog' | 'users'
export type NavigationItem = { id: WorkspaceSection; label: string; icon: string; adultsOnly?: boolean }
export const workspaceSections: NavigationItem[] = [
 { id: 'today', label: 'Мой день', icon: '☀️' },
 { id: 'day', label: 'Планер', icon: '🗓️' },
 { id: 'math', label: 'Математика', icon: '🔢' },
 { id: 'sport', label: 'Спорт', icon: '🏃' },
 { id: 'family', label: 'Приключения', icon: '🧭' },
 { id: 'earned', label: 'Награды', icon: '⭐' },
 { id: 'catalog', label: 'Обязанности', icon: '📚', adultsOnly: true },
 { id: 'users', label: 'Настройки', icon: '⚙️', adultsOnly: true },
]
