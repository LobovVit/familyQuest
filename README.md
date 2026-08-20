# FamilyQuest

FamilyQuest is a family task and reward tracker built with Go, PostgreSQL, React, and TypeScript.

## Local start

Docker with Docker Compose is the only prerequisite.

```bash
cp .env.example .env
# Replace POSTGRES_PASSWORD and SESSION_SECRET in .env first.
docker compose config --quiet
docker compose up --build -d
```

Open `http://localhost:8088`. The API listens on `127.0.0.1:8081` and PostgreSQL on `127.0.0.1:5433`; all published ports are loopback-only. Override `WEB_PORT`, `API_PORT`, or `POSTGRES_PORT` in `.env` if needed.

The development seed contains local-only sample accounts. Credentials are deliberately not documented or enabled in production. Change every sample PIN before placing an instance on a shared network.

```bash
docker compose ps
docker compose down
```

To delete the local database and re-import the development seed:

```bash
docker compose down -v
docker compose up --build -d
```

## Authentication and authorization

PIN verification creates a signed, time-limited session. Send the returned token as `Authorization: Bearer <token>` on protected API calls. Identity and role are derived from that token; client-supplied participant IDs are not an authorization boundary.

PINs are bcrypt hashes in new installations. Upgraded databases may temporarily retain a legacy PIN until its first successful login, when the application replaces it with a hash. Backups, participant management, chores, rewards, and confirmations are restricted according to server-side role policies.

`SESSION_SECRET` signs sessions and must be a high-entropy secret of at least 32 random bytes. Rotating it immediately invalidates existing sessions. `SESSION_TTL` defaults to `12h`.

## Configuration

Copy `.env.example` to `.env`; `.env` is ignored by Git. Important variables:

| Variable | Purpose |
| --- | --- |
| `POSTGRES_PASSWORD` | Required raw database password passed to PostgreSQL |
| `DATABASE_URL` | Required connection URL; URL-encode reserved password characters here |
| `SESSION_SECRET` | Required production session-signing secret, minimum 32 random bytes |
| `SESSION_TTL` | Session lifetime, for example `12h` |
| `CORS_ORIGIN` | Exact browser origin allowed in local development |
| `PUBLIC_HOST` | Production DNS host, without scheme |
| `TRAEFIK_NETWORK` | External Traefik Docker network |
| `TRAEFIK_CERT_RESOLVER` | Traefik ACME resolver name |

Generate secrets with your platform's secret manager or a cryptographically secure generator. Do not commit `.env`, paste secrets into Compose files, or reuse the database password as the session secret.

## Production deployment

Production does not publish PostgreSQL or API ports and does not load the development seed. It requires an existing Traefik network and exposes only HTTPS, with HTTP redirected to HTTPS.

```bash
cp .env.example .env
# Set unique POSTGRES_PASSWORD, SESSION_SECRET, PUBLIC_HOST and Traefik values.
docker compose -f docker-compose.prod.yml config --quiet
docker compose -f docker-compose.prod.yml up --build -d
```

Containers run with read-only root filesystems, dropped Linux capabilities, `no-new-privileges`, health checks, and dependency health gating. The application images run as non-root users.

## Database migrations

SQL migrations live in `backend/migrations` and are applied once, in filename order. The application records each successful migration in `schema_migrations`; do not edit an already deployed migration. Add a new numbered file instead.

Migration `002` introduces `pin_hash` while allowing a compatibility window for legacy `pin_code`. Removing the legacy column should be a later migration after all deployed credentials have been converted.

## Local development and checks

```bash
docker compose up -d postgres

cd backend
go test -race ./...
go vet ./...

cd ../frontend
npm ci
npm run lint
npm run build
npm test --if-present
```

GitHub Actions runs these backend and frontend checks and builds both container images on pushes and pull requests.

## Updating a home server

`scripts/update-home.sh` refuses a dirty checkout, fast-forwards the configured branch, validates the production Compose configuration, rebuilds the services, waits for their health status, and checks the public HTTPS endpoint. Configure deployment values in the server-side `.env`; never place secrets in the script.
