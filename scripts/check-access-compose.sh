#!/usr/bin/env bash
# Disposable topology acceptance; never targets the production project or its volumes.
# Одноразовая проверка топологии; не использует production-проект и его volumes.
set -euo pipefail
cd "$(dirname "$0")/.."
project="fq-access-check-$$"
network="${project}-external"
tmp="$(mktemp -d)"
compose=(docker compose --env-file "$tmp/env" -p "$project" -f docker-compose.prod.yml -f docker-compose.access.yml -f "$tmp/override.yml")
cleanup() {
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT
cat > "$tmp/env" <<ENV
POSTGRES_USER=postgres
POSTGRES_PASSWORD=compose-test-only
POSTGRES_DB=familyquest
DATABASE_URL=postgres://fq_core:core-test-only@postgres:5432/familyquest?sslmode=disable
ACCESS_DATABASE_URL=postgres://fq_access:access-test-only@postgres:5432/familyquest?sslmode=disable
SESSION_SECRET=compose-test-session-secret-00000000000000000000
ACCESS_SHARED_SECRET=compose-test-access-secret-00000000000000000000
PUBLIC_HOST=localhost
TRAEFIK_NETWORK=$network
ENV
cat > "$tmp/override.yml" <<'YAML'
services:
  web:
    labels:
      traefik.enable: "false"
    ports: ["127.0.0.1::8080"]
YAML
docker network create "$network" >/dev/null
"${compose[@]}" config --quiet
"${compose[@]}" build api access web
"${compose[@]}" up -d --wait postgres
operator() {
  "${compose[@]}" run --rm -T --no-deps -e DATABASE_URL='postgres://postgres:compose-test-only@postgres:5432/familyquest?sslmode=disable' api familyquest-operator "$1"
}
operator migrate
printf '%s\n' '{"Name":"Smoke family","Email":"owner@example.test","Password":"compose-password-test","Parent":"Parent","PIN":"739281","Operator":"smoke"}' | operator provision
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d familyquest <<'SQL'
create role fq_core login noinherit password 'core-test-only';
create role fq_access login noinherit password 'access-test-only';
SQL
printf '%s\n' '{"Role":"fq_core"}' | operator grant-core
printf '%s\n' '{"Role":"fq_access"}' | operator grant-access
"${compose[@]}" up -d --wait --wait-timeout 120
address="$("${compose[@]}" port web 8080)"
python3 - "$address" <<'PY'
import json,sys,urllib.request,urllib.error
base='http://'+sys.argv[1]
def call(path,body=None,token=None):
 headers={'Content-Type':'application/json','X-FamilyQuest':'1'}
 if token: headers['Authorization']='Bearer '+token
 req=urllib.request.Request(base+path,data=json.dumps(body).encode() if body is not None else None,headers=headers)
 try:
  with urllib.request.urlopen(req,timeout=10) as res: return res.status,json.load(res)
 except urllib.error.HTTPError as e: return e.code,None
assert call('/api/health')[0]==200
version=call('/api/version')[1]; assert version['access']['version']==version['version']
assert call('/api/participants')[0]==401
assert call('/api/__access/introspect')[0]==404
code,login=call('/api/account/login',{'email':'owner@example.test','password':'compose-password-test'})
assert code==200,code
token=login['token']
assert call('/api/participants',token=token)[0]==200
code,sub=call('/api/subscription',token=token)
assert code==200 and sub['paid'] is False and sub['access'] is False
assert call('/api/participants',{'name':'Child','role':'child','pin':'391827'},token)[0]==403
print('PASS: real proxy login, isolation boundary, subscription and versions')
PY
# The core refuses credentials-free requests even from its own container.
# Core отклоняет запрос без служебного ключа даже из собственного контейнера.
if "${compose[@]}" exec -T api wget -qO- http://127.0.0.1:8081/api/participants; then
  echo 'Core bypass detected' >&2; exit 1
fi
# The edge has no route to the private origin or database.
# Web не должен видеть private origin и PostgreSQL по Docker DNS.
if "${compose[@]}" exec -T web nslookup familyquest-core >/dev/null 2>&1; then
  echo 'Core visible from web' >&2; exit 1
fi
if "${compose[@]}" exec -T web nslookup postgres >/dev/null 2>&1; then
  echo 'Database visible from web' >&2; exit 1
fi
echo 'PASS: private core and network separation'
