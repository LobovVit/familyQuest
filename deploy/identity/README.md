# Общий сервер идентификации LOVIT

ZITADEL 4.16.0 и login UI работают на `10.66.66.6` в
`/home/lobov/lovit-identity`. Внешний Nginx `193.124.112.191` завершает TLS
для `auth.lovit.tech`, проксирует через VPN к Traefik на порту 80.
Traefik направляет `/ui/v2/login` в login UI, остальные запросы — в ZITADEL
по h2c. PostgreSQL доступна только в приватной Docker-сети.

## Статус

Инфраструктура запущена, три контейнера healthy, публичные HTTPS login и
OIDC discovery отвечают. Это ещё не завершённый SSO для FamilyQuest:
клиент, импорт владельца и сквозной вход предстоит настроить и проверить.
Production FamilyQuest продолжает прежний парольный вход. Универсальный
биллинг этим стеком не реализуется.

## Секреты и запуск

Файл `.env` (0600, не в Git) содержит:
- `POSTGRES_ADMIN_PASSWORD` — случайный hex-пароль администратора БД;
- `IDENTITY_DB_PASSWORD` — отдельный hex-пароль роли zitadel;
- `IDENTITY_MASTERKEY` — стабильный ключ шифрования, ровно 32 символа;
- `OPERATOR_PAT_EXPIRY` — RFC3339, bootstrap PAT создан на 30 дней;
- `LOGIN_PAT_EXPIRY` — RFC3339, PAT login UI создан на 365 дней.

```sh
docker compose up -d --wait
docker compose ps
```

Не генерируйте новый master key при обновлении существующей БД.
`init-db.sh` создаёт непривилегированную роль zitadel при первой инициализации.
Bootstrap запускается от UID 0 без capabilities, чтобы писать в новые volumes;
login получает только login PAT volume, административный PAT ему не доступен.
Перед эксплуатацией необходимо назначить ответственного за обновление PAT до
истечения срока. Простая смена даты в .env не перевыпускает существующий PAT.

## Сохранность и восстановление

Сохраняйте dump БД zitadel, защищённую .env/master key и оба bootstrap volumes
в закрытую резервную копию. Копия без master key недостаточна для восстановления.
До подключения пользовательских аккаунтов необходимо проверить восстановление.
Не применять `docker compose down -v` к рабочему экземпляру.
Теги образов закреплены; автоматическое обновление Watchtower отключено.

Сертификат Let's Encrypt находится на VPS в `/etc/letsencrypt/live/auth.lovit.tech`.
Продление выполняет certbot.timer через webroot `/var/www/certbot`, deploy hook
`20-nginx-reload` проверяет и перечитывает Nginx. Соседний `lovit.tech` — отдельный
репозиторий и сервис; изменение его сайта не требует перезапуска identity.
