#!/usr/bin/env bash
# Copyright 2026 Alibaba Group
# Licensed under the Apache License, Version 2.0
#
# Mirror release artifacts to a Gitee release so China users can install without
# hitting GitHub. The repo code itself is kept in sync by Gitee's repo-mirror
# feature; this script handles what the mirror does NOT carry — the GitHub
# Release *attachments* (binaries, checksums, skills zip) — by uploading them to
# the matching Gitee release via the Gitee OpenAPI v5.
#
# Consumed by install.sh when DWS_GITEE_REPO is set (it resolves each asset's
# real download_url from the Gitee API, since Gitee attachment URLs carry an
# unstable numeric id).
#
# Required environment (CI secrets):
#   GITEE_TOKEN   Gitee private access token (scopes: projects)
#   GITEE_REPO    "owner/repo" on Gitee, e.g. DingTalk-Real-AI/dingtalk-workspace-cli
# Optional:
#   VERSION       release tag (default: git describe)
#   DIST_DIR      artifacts dir (default: ./dist)
#   GITEE_API     API base (default: https://gitee.com/api/v5)
#
# Gating: if GITEE_TOKEN / GITEE_REPO are unset, exit 0 with a notice so the
# step can live in release.yml without breaking forks that lack the secret.

set -eu

DIST_DIR="${DIST_DIR:-dist}"
GITEE_API="${GITEE_API:-https://gitee.com/api/v5}"

missing=""
[ -z "${GITEE_TOKEN:-}" ] && missing="$missing GITEE_TOKEN"
[ -z "${GITEE_REPO:-}" ]  && missing="$missing GITEE_REPO"
if [ -n "$missing" ]; then
  echo "ℹ️  Gitee mirror sync skipped — missing:${missing}"
  echo "   Set these as repo secrets to auto-mirror releases to Gitee for China users."
  exit 0
fi

VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"
OWNER="${GITEE_REPO%%/*}"
NAME="${GITEE_REPO##*/}"
base="${GITEE_API}/repos/${OWNER}/${NAME}"

echo "📦 Mirroring release ${VERSION} → Gitee ${GITEE_REPO}"

# ── Resolve or create the Gitee release for this tag ──────────────────────────
# Gitee mirror sync brings the git tag over, so the tag should already exist.
rel_json="$(curl -fsSL "${base}/releases/tags/${VERSION}?access_token=${GITEE_TOKEN}" 2>/dev/null || true)"
release_id="$(printf '%s' "$rel_json" | grep -o '"id":[ ]*[0-9]*' | head -1 | grep -o '[0-9]*' || true)"

if [ -z "$release_id" ]; then
  echo "   No Gitee release for ${VERSION} yet — creating it."
  rel_json="$(curl -fsSL -X POST "${base}/releases" \
    -F "access_token=${GITEE_TOKEN}" \
    -F "tag_name=${VERSION}" \
    -F "name=${VERSION}" \
    -F "body=Mirror of GitHub release ${VERSION} for China users." \
    -F "target_commitish=main" 2>/dev/null || true)"
  release_id="$(printf '%s' "$rel_json" | grep -o '"id":[ ]*[0-9]*' | head -1 | grep -o '[0-9]*' || true)"
fi
[ -n "$release_id" ] || { echo "❌ Could not get/create Gitee release for ${VERSION}. Response: ${rel_json}" >&2; exit 1; }
echo "   Gitee release id = ${release_id}"

# ── Names already attached to this Gitee release (for idempotent re-runs) ─────
# Re-fetch the release so the asset list reflects a release we may have just
# created. Skipping already-present files makes a re-run upload only what is
# missing — so a previous run that timed out mid-upload can be repaired cheaply
# instead of re-uploading everything (and creating duplicate attachments).
existing_names="$(curl -fsSL "${base}/releases/${release_id}?access_token=${GITEE_TOKEN}" 2>/dev/null \
  | python3 -c 'import json,sys
try:
    print("\n".join(a.get("name","") for a in json.load(sys.stdin).get("assets",[])))
except Exception:
    pass' 2>/dev/null || true)"

# ── Upload each artifact as a release attachment ──────────────────────────────
uploaded=0
skipped=0
for f in "$DIST_DIR"/dws-*.tar.gz "$DIST_DIR"/dws-*.zip "$DIST_DIR"/checksums.txt; do
  [ -f "$f" ] || continue
  fn="$(basename "$f")"
  if printf '%s\n' "$existing_names" | grep -qxF "$fn"; then
    echo "   ✓ ${fn} already on Gitee — skip"
    skipped=$((skipped + 1))
    continue
  fi
  echo "   ⬆ ${fn}"
  resp="$(curl -fsSL -X POST "${base}/releases/${release_id}/attach_files" \
    -F "access_token=${GITEE_TOKEN}" \
    -F "file=@${f}" 2>/dev/null || true)"
  if printf '%s' "$resp" | grep -q '"browser_download_url"'; then
    uploaded=$((uploaded + 1))
  else
    echo "   ⚠ upload may have failed for ${fn}: ${resp}" >&2
  fi
done

if [ "$uploaded" -eq 0 ] && [ "$skipped" -eq 0 ]; then
  echo "❌ No artifacts found to upload. Did the build (goreleaser) run / were assets downloaded into ${DIST_DIR}?" >&2
  exit 1
fi
echo "✅ Gitee release ${VERSION}: uploaded ${uploaded} asset(s), skipped ${skipped} already present."
echo "   China install:  DWS_GITEE_REPO=${GITEE_REPO} \\"
echo "     curl -fsSL https://gitee.com/${GITEE_REPO}/raw/main/scripts/install.sh | sh"
