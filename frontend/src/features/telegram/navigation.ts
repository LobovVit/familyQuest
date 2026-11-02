import type { WorkspaceSection } from '../navigation/sections'
const routes:Record<string,WorkspaceSection>={tasks:'day',review:'day',assign:'catalog',rewards:'day',settings:'users'}
export function telegramSection(start:string):WorkspaceSection { return Object.hasOwn(routes,start) ? routes[start] : 'day' }
