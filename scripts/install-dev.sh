#!/bin/sh
# Copyright 2026 Alibaba Group
# Licensed under the Apache License, Version 2.0
#
# One-command installer for the Dev preview branch.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/wxianfeng/dingtalk-workspace-cli/feat/dws-dev/scripts/install-dev.sh | sh
#
# Environment variables:
#   DWS_DEV_REPO_URL    Git repository URL. Default: https://github.com/wxianfeng/dingtalk-workspace-cli.git
#   DWS_DEV_BRANCH      Branch to install. Default: feat/dws-dev
#   DWS_DEV_SOURCE_DIR  Existing source checkout to install from.
#   DWS_DEV_KEEP_SOURCE Set to 1 to keep the temporary source checkout.
#
# The script clones the requested branch and delegates to scripts/install.sh
# from that checkout, which builds and installs dws from local source.

set -eu

REPO_URL="${DWS_DEV_REPO_URL:-https://github.com/wxianfeng/dingtalk-workspace-cli.git}"
BRANCH="${DWS_DEV_BRANCH:-feat/dws-dev}"
SOURCE_DIR="${DWS_DEV_SOURCE_DIR:-}"
KEEP_SOURCE="${DWS_DEV_KEEP_SOURCE:-0}"
TMPDIR_WORK=""

say() {
  printf '  %s\n' "$@"
}

err() {
  printf '  ERROR: %s\n' "$@" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "Missing required command: $1"
}

cleanup() {
  if [ -n "$TMPDIR_WORK" ] && [ "$KEEP_SOURCE" != "1" ]; then
    rm -rf "$TMPDIR_WORK"
  fi
}

trap cleanup EXIT INT TERM

print_banner() {
  printf '\n'
  say "Dev preview installer"
  say "Repository: ${REPO_URL}"
  say "Branch:     ${BRANCH}"
  printf '\n'
}

resolve_source_dir() {
  if [ -n "$SOURCE_DIR" ]; then
    [ -d "$SOURCE_DIR" ] || err "DWS_DEV_SOURCE_DIR does not exist: ${SOURCE_DIR}"
    [ -f "$SOURCE_DIR/scripts/install.sh" ] || err "install.sh not found under DWS_DEV_SOURCE_DIR: ${SOURCE_DIR}"
    printf '%s\n' "$SOURCE_DIR"
    return 0
  fi

  need_cmd git

  TMPDIR_WORK="$(mktemp -d)"
  checkout="${TMPDIR_WORK}/dingtalk-workspace-cli"

  say "Cloning Dev preview source..." >&2
  git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$checkout"

  [ -f "$checkout/scripts/install.sh" ] || err "install.sh not found after clone"
  printf '%s\n' "$checkout"
}

main() {
  print_banner

  if [ "${DWS_SKILLS_ONLY:-0}" != "1" ]; then
    need_cmd go
    need_cmd make
  fi

  source_root="$(resolve_source_dir)"

  say ""
  say "Installing dws from Dev preview source..."
  # the dev skill (dingtalk-dev) ships only in multi mode (developer-facing); mono targets
  # general office users and intentionally has no dev routing.
  DWS_VERSION=latest DWS_SKILL_MODE="${DWS_SKILL_MODE:-multi}" sh "$source_root/scripts/install.sh"

  say ""
  say "Dev next steps:"
  say "  dws version"
  say "  dws auth login"
  say "  dws skill setup --mode multi   # install per-product skills incl. dingtalk-dev"
  say "  dws dev --help --format json"
  say "  dws dev app list --format json"

  if [ "$KEEP_SOURCE" = "1" ]; then
    say ""
    say "Source checkout kept at: ${source_root}"
  fi
}

main
