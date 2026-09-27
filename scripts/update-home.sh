#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/familyQuest}"
BRANCH="${BRANCH:-main}"
RELEASE_TAG="${RELEASE_TAG:-}"
PROJECT="${PROJECT:-familyquest}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
HEALTH_HOST="${HEALTH_HOST:-lobov.family}"
HEALTH_URL="${HEALTH_URL:-https://${HEALTH_HOST}/api/health}"

cd "$APP_DIR"

echo "==> Updating $APP_DIR from origin/$BRANCH"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Local changes found. Commit/stash them before deployment." >&2
  git status --short >&2
  exit 1
fi

git fetch origin "$BRANCH" --tags
if [[ -n "$RELEASE_TAG" ]]; then
  [[ "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo 'Invalid release tag' >&2; exit 1; }
  [[ "$(git cat-file -t "refs/tags/$RELEASE_TAG")" == tag ]] || { echo 'Release must use an annotated tag' >&2; exit 1; }
  [[ "$(git show "refs/tags/$RELEASE_TAG:VERSION")" == "${RELEASE_TAG#v}" ]] || { echo 'Tag and VERSION differ' >&2; exit 1; }
  git checkout --detach "refs/tags/$RELEASE_TAG"
else
  git checkout "$BRANCH"
  git pull --ff-only origin "$BRANCH"
fi
source scripts/build-metadata.sh
echo "==> Build $APP_VERSION ($APP_COMMIT)"

compose=(docker compose -p "$PROJECT" -f "$COMPOSE_FILE")
services=(api web)
if [[ -f .access-proxy-enabled ]]; then
  [[ -f docker-compose.access.yml ]] || { echo 'This checkout cannot deploy the configured access proxy' >&2; exit 1; }
  compose+=(-f docker-compose.access.yml)
  services+=(access)
fi
echo "==> Building and restarting Docker Compose project: $PROJECT"
"${compose[@]}" config --quiet
"${compose[@]}" up --build -d --remove-orphans

echo "==> Containers"
"${compose[@]}" ps

echo "==> Waiting for services to become healthy"
"${compose[@]}" up -d --wait --wait-timeout "${WAIT_TIMEOUT:-120}"

echo "==> Checking public HTTPS endpoint: $HEALTH_URL"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$HEALTH_URL" >/dev/null

echo "==> Verifying running API and web build identity"
VERSION_URL="${VERSION_URL:-https://${HEALTH_HOST}/api/version}"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$VERSION_URL" | python3 -c 'import json,os,sys; v=json.load(sys.stdin); assert v["commit"]==os.environ["APP_COMMIT"] and v["version"]==os.environ["APP_VERSION"], "Unexpected API build"; print(v["version"],v["commit"])'
for service in "${services[@]}"; do
  container="$("${compose[@]}" ps -q "$service")"
  [[ "$(docker inspect "$container" --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}')" == "$APP_COMMIT" ]] || { echo "Unexpected $service image" >&2; exit 1; }
done
echo "==> Done"
