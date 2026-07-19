# FamilyQuest

FamilyQuest is a family task and reward tracker for teaching planning, responsibility, and feedback.

## Stack

- Backend: Go
- Database: PostgreSQL
- Frontend: Vite + React + TypeScript

## First Run

Assume nothing is installed locally except Docker with Docker Compose support. Go, Node.js, npm, Vite, nginx, and PostgreSQL all run inside containers.

Start the whole MVP from the repository root:

```bash
docker compose up --build -d
```

Open the app at `http://localhost:8088`.

Services:

- Frontend: `http://localhost:8088`
- Backend API: `http://localhost:8081`
- PostgreSQL: `localhost:5433`

If one of these ports is already busy, override it for this run:

```bash
WEB_PORT=18088 API_PORT=18081 POSTGRES_PORT=15433 docker compose up --build -d
```

Seeded PIN codes:

- Мама: `111111`
- Папа: `222222`
- Макс: `333333`

Stop the app:

```bash
docker compose down
```

Reset local data and start from the seed again:

```bash
docker compose down -v
docker compose up --build -d
```

## Optional Local Development

Use this only if you already have Go and Node.js installed locally. The Docker path above is the default path.

1. Start PostgreSQL:

```bash
docker compose up -d postgres
```

2. Start the backend:

```bash
cd backend
DATABASE_URL=postgres://familyquest:familyquest@localhost:5433/familyquest?sslmode=disable HTTP_ADDR=:8081 go run ./cmd/api
```

3. Start the frontend:

```bash
cd frontend
npm install
npm run dev
```

The frontend expects the API at `http://localhost:8081`. Override it with `VITE_API_URL` if needed.

## Current MVP

- Seeded participants: Мама, Папа, Макс
- Seeded first chore catalog for a family with a 6-year-old preschool child
- Initial assignments for all three family members
- Chore schedules: once, daily, weekly, monthly
- Separate "when" choice for a chore: no window, morning, day, evening
- Assign chores to participants
- Generate tasks for the selected day from active assignments
- Mark tasks as completed
- Confirm tasks with a 1-5 rating
- Reward formula: `base_value * average_rating / 5`
- Daily and weekly leaderboard
