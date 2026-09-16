#!/bin/sh
set -eu

# Check open-source assets: wrapper around open-source-audit.sh
# This script exists for backward compatibility with Makefile, CONTRIBUTING.md,
# and maintainer automation doc references.

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
exec "$ROOT/scripts/policy/open-source-audit.sh" "$@"
