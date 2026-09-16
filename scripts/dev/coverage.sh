#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
PROFILE="${1:-$ROOT/coverage.out}"

cd "$ROOT"
go test -coverprofile="$PROFILE" ./...
go tool cover -func="$PROFILE" | tail -n 1
printf 'coverage profile: %s\n' "$PROFILE"
