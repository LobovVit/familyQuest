# FamilyQuest

FamilyQuest is a family task and reward tracker for teaching planning, responsibility, and feedback.

## Stack

- Backend: Go
- Database: PostgreSQL
- Frontend: Vite + React + TypeScript

## Local Run

### Full Docker run

Start the whole MVP:

```bash
docker compose up --build
```

Open the app at `http://localhost:8088`.

Services:

- Frontend: `http://localhost:8088`
- Backend API: `http://localhost:8081`
- PostgreSQL: `localhost:5433`

### Local development

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
- Chore schedules: once, daily, daily with time window, weekly, monthly
- Assign chores to participants
- Generate tasks for the selected day from active assignments
- Mark tasks as completed
- Confirm tasks with a 1-5 rating
- Reward formula: `base_value * average_rating / 5`
- Weekly and monthly leaderboard
