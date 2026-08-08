#!/bin/sh
set -eu

# B161 Phase I prototype: json-mode stdout must be pure JSON.
#
# Contract anchors: AC-11 (json 模式 stdout 零日志字节) / AC-07. For every
# sample command the captured stdout must be non-empty, parse as exactly one
# JSON document (jq -s), and contain no log-prefix lines ([INFO]/[WARN]/...)
# or ANSI escape bytes. Diagnostics belong on stderr.
#
# This is a PROTOTYPE scan, not a wired policy gate. Positioning, sample
# selection, the B164 false-positive verification record, --scope semantics
# (B166) and the `make policy` hook design draft (B167) live in
# scripts/policy/README.md. Samples run under an isolated fresh HOME
# (DWS_SCAN_HOME overrides), so the default dev scope needs no login.
#
# Usage:
#   check-stdout-json.sh [--scope dev|all]   (default: dev)
#   check-stdout-json.sh --self-test         scan runtime-generated fixtures

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
. "$ROOT/scripts/policy/unified-result-lib.sh"

OC_SCRIPT_NAME="stdout-json"

unified_result_usage() {
	printf 'usage: %s [--scope dev|all] | --self-test | --help\n' "$0"
}

stdout_json_scan() {
	stdout_json_class="$1"
	stdout_json_label="$2"
	stdout_json_out="$3"
	# stderr is captured by the harness but this check only judges stdout.
	stdout_json_violations=""

	if [ ! -s "$stdout_json_out" ]; then
		stdout_json_violations="stdout is empty (json-mode success output must be one JSON document)"
	else
		stdout_json_doc_count="$(jq -s 'length' <"$stdout_json_out" 2>/dev/null)" || {
			stdout_json_violations="stdout is not parseable JSON (first bytes: $(head -c 80 "$stdout_json_out" | tr '\n' ' '))"
			stdout_json_doc_count=""
		}
		if [ -n "$stdout_json_doc_count" ] && [ "$stdout_json_doc_count" != "1" ]; then
			stdout_json_violations="${stdout_json_violations:+$stdout_json_violations; }stdout contains $stdout_json_doc_count JSON documents (expected exactly 1; log lines or double emits break pipes)"
		fi
		stdout_json_log_lines="$(grep -inE '^[[:space:]]*\[(INFO|WARN|WARNING|ERROR|ERR|OK|DEBUG|TRACE|FATAL|PANIC|connect|pagination|deprecation)\]' "$stdout_json_out" || true)"
		if [ -n "$stdout_json_log_lines" ]; then
			stdout_json_violations="${stdout_json_violations:+$stdout_json_violations; }log-prefix line(s) on stdout: $(printf '%s' "$stdout_json_log_lines" | head -3 | tr '\n' '|')"
		fi
		stdout_json_esc="$(printf '\033')"
		stdout_json_ansi="$(LC_ALL=C grep -n "$stdout_json_esc" "$stdout_json_out" || true)"
		if [ -n "$stdout_json_ansi" ]; then
			stdout_json_violations="${stdout_json_violations:+$stdout_json_violations; }ANSI escape byte(s) on stdout (line(s): $(printf '%s' "$stdout_json_ansi" | cut -d: -f1 | head -3 | tr '\n' ' '))"
		fi
	fi

	unified_result_report_violations "$stdout_json_label" "$stdout_json_violations"
}

unified_result_init "$ROOT"
unified_result_parse_args "$@"

self_test_cases() {
	printf 'envelope_legal_success|envelope|pass\n'
	printf 'envelope_legal_pending_dry_run|envelope|pass\n'
	printf 'envelope_legal_partial_failure|envelope|pass\n'
	printf 'envelope_legal_failure|envelope|pass\n'
	printf 'envelope_ok_full_meta|envelope|pass\n'
	printf 'stdout_polluted_info|envelope|fail\n'
	printf 'stdout_log_polluted|envelope|fail\n'
	printf 'stdout_two_documents|envelope|fail\n'
	printf 'stdout_ansi_escape|envelope|fail\n'
	printf 'stdout_empty|envelope|fail\n'
}

if [ "$SELF_TEST" -eq 1 ]; then
	unified_result_run_self_test stdout_json_scan
fi

unified_result_scan_samples stdout_json_scan
