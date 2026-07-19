#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/familyQuest}"
BRANCH="${BRANCH:-main}"
PROJECT="${PROJECT:-familyquest}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
HEALTH_HOST="${HEALTH_HOST:-lobov.family}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1}"

cd "$APP_DIR"

echo "==> Updating $APP_DIR from origin/$BRANCH"

if [[ -n "$(git status --porcelain)" && "${ALLOW_DIRTY:-0}" != "1" ]]; then
  echo "Local changes found. Commit/stash them first, or run with ALLOW_DIRTY=1." >&2
  git status --short >&2
  exit 1
fi

git fetch origin "$BRANCH"
git checkout "$BRANCH"
git pull --ff-only origin "$BRANCH"

echo "==> Building and restarting Docker Compose project: $PROJECT"
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" up --build -d --remove-orphans

echo "==> Containers"
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" ps

echo "==> Checking Traefik route: Host: $HEALTH_HOST -> $HEALTH_URL"
curl -fsSI -H "Host: $HEALTH_HOST" "$HEALTH_URL" >/dev/null

echo "==> Done"
