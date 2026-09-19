# FamilyQuest frontend

React + TypeScript + Vite. Пользовательские сценарии: планер и рейтинги, управление обязанностями/участниками/наградами, [семейные занятия и привычки](../docs/family-life.md).

```bash
npm ci
npm run dev
```

Dev-server проксирует `/api` согласно `vite.config.ts`. Backend должен быть запущен отдельно. `VITE_API_URL` можно задать при сборке для отдельного API origin; в Docker используется относительный `/api` через Nginx. Переменные `VITE_*` попадают в браузер, секреты в них недопустимы.

```bash
npm run lint
npm test
npm run build
```

`src/domain` содержит модели и правила; `src/application` — hooks и порты; `src/infrastructure` — HTTP, sessionStorage и backup; `src/features`/`src/pages` — интерфейс. Реализации собираются в `src/main.tsx` через `RuntimeContext`.

Тесты проверяют политики, сессии, гонки асинхронной загрузки и границы зависимостей. DOM-проверки выполняются через jsdom/Testing Library. Итоговая сборка находится в `dist`. Общая [архитектура](../docs/architecture.md), [API](../docs/api.md), [эксплуатация](../docs/operations.md).
