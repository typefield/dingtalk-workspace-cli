#!/bin/sh
set -eu

# Ensure catalog confirmation=user_required exactly matches the reviewed
# runtime gate set in schema_hints/index.json (gate != none).

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$ROOT"

truth="internal/cli/schema_hints/index.json"
catalog="internal/cli/schema_catalog.json"

if [ ! -f "$truth" ]; then
	printf '%s\n' "missing agent hint index: $truth" >&2
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

jq -r '
  .runtime_gates
  | to_entries[]
  | select(.value != "none")
  | .key
' "$truth" | sort >"$tmp/truth_gated"

jq -r '
  .tools
  | to_entries[]
  | select(.value.confirmation == "user_required")
  | .key
' "$catalog" | sort >"$tmp/catalog_required"

if ! cmp -s "$tmp/truth_gated" "$tmp/catalog_required"; then
	printf '%s\n' 'catalog confirmation=user_required differs from schema_hints/index.json runtime_gates gate!=none set' >&2
	printf '%s\n' 'update internal/cli/schema_hints/products/<product>.json and index.json runtime_gates, then regenerate schema' >&2
	diff -u "$tmp/truth_gated" "$tmp/catalog_required" || true
	exit 1
fi

if ! jq -e --slurpfile catalog "$catalog" '
  . as $truth |
  ($catalog[0].tools) as $tools |
  all($truth.runtime_gates | to_entries[];
    .key as $canonical |
    .value as $gate |
    ($tools[$canonical] != null) and (
      if $gate == "none" then
        $tools[$canonical].confirmation == "not_required" and
        $tools[$canonical].risk != "high" and
        $tools[$canonical].effect != "destructive"
      else
        $tools[$canonical].confirmation == "user_required"
      end
    )
  )
' "$truth" >/dev/null; then
	printf '%s\n' 'schema_hints/index.json runtime_gates disagree with catalog effect/risk/confirmation' >&2
	jq -r --slurpfile catalog "$catalog" '
	  . as $truth |
	  ($catalog[0].tools) as $tools |
	  $truth.runtime_gates
	  | to_entries[]
	  | .key as $canonical
	  | .value as $gate
	  | $tools[$canonical] as $tool
	  | select(
	      $tool == null or (
	        if $gate == "none" then
	          $tool.confirmation != "not_required" or $tool.risk == "high" or $tool.effect == "destructive"
	        else
	          $tool.confirmation != "user_required"
	        end
	      )
	    )
	  | "\(.key)\tgate=\(.value)\teffect=\($tool.effect // "MISSING")\trisk=\($tool.risk // "MISSING")\tconfirmation=\($tool.confirmation // "MISSING")"
	' "$truth" >&2
	exit 1
fi

gated_count="$(wc -l <"$tmp/truth_gated" | tr -d ' ')"
printf 'runtime confirmation truth: ok (%s gated user_required commands)\n' "$gated_count"
