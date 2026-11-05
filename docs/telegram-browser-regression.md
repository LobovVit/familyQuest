# Изолированная HTTPS-проверка cookies

`TestTelegramCrossSiteHTTPSBrowser` запускает настоящие FamilyQuest Access, Core
и отдельную PostgreSQL. Chromium обращается к ним по HTTPS через loopback:
`product.test` встроен в iframe на другом сайте `container.test`.
Подмена ответов через Playwright `route.fulfill` и ручная установка cookies не
используются. HTML-оболочка, внешний Portal и IdP — тестовые имитации.

Проверяется текущая реализация `Partitioned=t.Secure`; runtime не изменён:

- В Chromium принудительно блокируются сторонние cookies. Отрицательный контроль
  удаляет **только** `Partitioned` из настоящего `Set-Cookie` Access: exchange
  отвечает 200, следующая проверка сессии — 401. CDP обязан зафиксировать причину
  блокировки cookie; без этого тест не проходит.
- Portal-имитация проверяет HMAC синтетического `initData` и повторное применение.
  Неверная подпись даёт 401, повтор успешного exchange — 409.
- С исходной cookie проходят выбор семьи, PIN и защищённый Core API, в том числе
  после reload. Под другим верхним сайтом та же сессия недоступна.
- Выход через клиентский `/api/session/logout` удаляет partitioned cookie;
  прежний bearer больше не даёт доступа к Telegram API.
- В том же контексте выполняется обычный OIDC redirect/PKCE/signed ID-token/PIN
  flow. После Telegram logout его cookie и API остаются рабочими; OIDC cookie
  сохраняет `SameSite=Lax`, `Secure`, `HttpOnly` и отсутствие `Partitioned`.

## Запуск

Нужны Go из `backend/go.mod`, Node 24, Docker. База обязана иметь имя
`familyquest_browser_test` и адрес `127.0.0.1`: тест откажется от другого адреса.
Никакие production URL, токены, профили браузера или пользовательский GUI не нужны.

```sh
npm ci --prefix scripts/browser --ignore-scripts
npx --prefix scripts/browser playwright install chromium
docker run --rm -d --name familyquest-browser-test \
  -e POSTGRES_PASSWORD=browser-fixture-only -e POSTGRES_DB=familyquest_browser_test \
  -p 127.0.0.1:55444:5432 postgres:17-alpine
# Дождаться "accepting connections":
docker exec familyquest-browser-test pg_isready -U postgres -d familyquest_browser_test
env -u GOROOT FAMILYQUEST_BROWSER_TEST=1 \
  FAMILYQUEST_TEST_DATABASE_URL='postgres://postgres:browser-fixture-only@127.0.0.1:55444/familyquest_browser_test?sslmode=disable' \
  go -C backend test -race ./internal/store -run '^TestTelegramCrossSiteHTTPSBrowser$' -count=1 -v
docker stop familyquest-browser-test
```

`FAMILYQUEST_BROWSER_EXECUTABLE` позволяет дополнительно проверить установленный
Chrome; по умолчанию используется Chromium из закреплённого Playwright.
`FAMILYQUEST_BROWSER_REPORT` — необязательный путь к JSON-отчёту без секретов.
Семья, схема и роль БД удаляются и при падении проверки; контейнер после локального
запуска нужно остановить командой выше. CI job `telegram-browser` создаёт свою
одноразовую БД и сохраняет отчёт как artifact.

Playwright по умолчанию отключает `ThirdPartyStoragePartitioning`; harness
возвращает этот флаг, сохраняя остальные настройки. Поддерживаются отдельный
CDP target iframe и общий с родителем target Headless Shell. В обоих случаях
обязателен наблюдаемый отрицательный контроль. Self-signed TLS разрешён только
в одноразовых браузерных контекстах теста.

Проверка не заменяет приёмку настоящего Telegram SDK/WebView, production Portal,
BotFather, opt-in, `/stop` или доставки. Она не включает Telegram, worker либо
согласие на сообщения и не требует изменения развёрнутого runtime.
