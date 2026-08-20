import type { BenefitType, Participant, RewardPeriod, RewardType, Schedule, TimeWindow } from '../../domain/models'
export const scheduleLabels:Record<Schedule,string>={once:'Разово',daily:'Ежедневно',weekly:'Еженедельно',monthly:'Ежемесячно'}
export const windowLabels:Record<TimeWindow,string>={'':'Когда угодно',morning:'Утро',day:'День',evening:'Вечер'}
export const benefitLabels:Record<BenefitType,string>={self:'Для себя',family:'Для семьи',care:'Забота',home:'Дом'}
export const roleLabels:Record<Participant['role'],string>={parent:'Взрослый',child:'Дошкольник',school:'Школьник'}
export const rewardPeriodLabels:Record<RewardPeriod,string>={day:'День',week:'Неделя',month:'Месяц'}
export const rewardTypeLabels:Record<RewardType,string>={champion:'Чемпионская',stars:'За звездочки',smiles:'За улыбки'}
