export type MathSettings = { operation: '+' | '-' | '*' | ':'; level: 'easy' | 'medium' | 'hard' | 'columnar'; answerMode: 'input' | 'choice'; divisionMode: 'result' | 'steps' | 'full'; allowNegative: boolean }
export type MathQuestion = { left: number; right: number; options: number[] }
export type MathField = { key: string; label: string; width: number; shift: number }
export type MathView = { finished: boolean; id: string; settings: MathSettings; index: number; total: number; correct: number; stars: number; bestStreak: number; createdAt: string; question?: MathQuestion; fields: MathField[]; feedback?: { index: number; correct: boolean; stars: number; expected: Record<string, number>; fields: MathField[]; question: MathQuestion } }
export type ActivityReward = { source: 'math' | 'sport' | 'habit' | 'adventure'; sourceKey: string; participantId: number; date: string; stars: number; smiles: number; title: string }
export const mathSymbols = { '+': '+', '-': '−', '*': '×', ':': ':' }
export const mathLevels = { easy: 'Лёгкий · до 10', medium: 'Средний · до 25', hard: 'Сложный · до 100', columnar: 'В столбик' }
