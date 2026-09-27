#!/usr/bin/env bash
# Source before building both images. / Подключать перед сборкой обоих образов.
set -euo pipefail
APP_VERSION="$(cat VERSION)"
[[ "$APP_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo 'Invalid VERSION' >&2; exit 1; }
APP_COMMIT="$(git rev-parse HEAD)"
if [[ "$(git describe --tags --exact-match HEAD 2>/dev/null || true)" != "v$APP_VERSION" ]]; then
 APP_VERSION="$APP_VERSION-dev.${APP_COMMIT:0:12}"
fi
if [[ -n "$(git status --porcelain)" ]]; then APP_VERSION="$APP_VERSION-dirty"; APP_COMMIT="$APP_COMMIT-dirty"; fi
APP_BUILT_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
export APP_VERSION APP_COMMIT APP_BUILT_AT
