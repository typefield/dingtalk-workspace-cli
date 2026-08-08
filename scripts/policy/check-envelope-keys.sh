#!/bin/sh
set -eu

# B163 Phase I prototype: non-standard envelope key scan (G1).
#
# Contract anchor: the envelope is an object with required ok/outcome fields,
# their I1/I3 invariants, and a fixed top-level key set —
# ok / outcome / identity / dry_run / data / meta / error / _notice
# (snake_case). This scan flags historical variants on envelope-class output:
#   - legacy status keys at the top level: success / errcode / error_code /
#     errorCode / err_code / isSuccess / is_success
#   - camelCase wire keys recursively under meta/error (business data is
#     excluded from this structural naming rule)
#
# legacy-class samples (pre-migration non-envelope json, e.g. schema list /
# auth status) are exempt from the envelope-shape scan — that is their known
# pre-migration shape, not a regression. See README.md.
#
# This is a PROTOTYPE scan, not a wired policy gate. Positioning, sample
# selection, the B164 false-positive verification record, --scope semantics
# (B166) and the `make policy` hook design draft (B167) live in
# scripts/policy/README.md. Samples run under an isolated fresh HOME
# (DWS_SCAN_HOME overrides), so the default dev scope needs no login.
#
# Usage:
#   check-envelope-keys.sh [--scope dev|all]   (default: dev)
#   check-envelope-keys.sh --self-test         scan runtime-generated fixtures

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
. "$ROOT/scripts/policy/unified-result-lib.sh"

OC_SCRIPT_NAME="envelope-keys"

unified_result_usage() {
	printf 'usage: %s [--scope dev|all] | --self-test | --help\n' "$0"
}

envelope_keys_scan() {
	envelope_keys_class="$1"
	envelope_keys_label="$2"
	envelope_keys_out="$3"
	envelope_keys_violations=""

	if [ ! -s "$envelope_keys_out" ]; then
		envelope_keys_violations="stdout is empty; nothing to scan"
		unified_result_report_violations "$envelope_keys_label" "$envelope_keys_violations"
		return 0
	fi

	if [ "$envelope_keys_class" = "legacy" ]; then
		# Known pre-migration non-envelope shape; envelope-key scan not applicable.
		return 0
	fi

	if ! jq empty <"$envelope_keys_out" >/dev/null 2>&1; then
		envelope_keys_violations="stdout is not parseable JSON; envelope-key scan fails closed (parseability gate: check-stdout-json.sh)"
		unified_result_report_violations "$envelope_keys_label" "$envelope_keys_violations"
		return 0
	fi

	# I5 deliberately defines no runtime contract marker. A valid envelope is
	# identified by its object shape and mandatory ok/outcome pair.
	envelope_keys_shape="$(jq -sr '
		def issue($message): $message;
		if length != 1 then
			issue("stdout must contain exactly one envelope document")
		else .[0] as $doc | $doc |
		if type != "object" then
			issue("envelope must be a JSON object")
		else
			(if has("ok") | not then issue("required top-level key missing: ok") else empty end),
			(if has("outcome") | not then issue("required top-level key missing: outcome") else empty end),
			(if has("ok") and (.ok | type) != "boolean" then issue("top-level ok must be a JSON boolean") else empty end),
			(if has("outcome") and ((.outcome | type) != "string" or (["success", "pending", "partial_failure", "failure"] | index($doc.outcome) | not))
			 then issue("top-level outcome must be one of success, pending, partial_failure, failure") else empty end),
			(if has("ok") and (.ok | type) == "boolean" and has("outcome") and (["success", "pending", "partial_failure", "failure"] | index($doc.outcome)) != null
			 and (.ok != (.outcome == "success" or .outcome == "pending"))
			 then issue("invariant I1 violated: ok must be true exactly for success or pending") else empty end),
			(if has("outcome") and (["success", "pending", "partial_failure", "failure"] | index($doc.outcome)) != null
			 and (has("error") != (.outcome == "failure"))
			 then issue("invariant I3 violated: error must be present exactly for failure") else empty end),
			(if has("error") and (.error | type) != "object" then issue("top-level error must be an object") else empty end),
			(if has("data") and has("error") then issue("data and error must be mutually exclusive") else empty end)
		end end' <"$envelope_keys_out")" || envelope_keys_shape="envelope shape validation failed closed"
	if [ -n "$envelope_keys_shape" ]; then
		envelope_keys_violations="$envelope_keys_shape"
	fi

	# Top-level key set: only the fixed envelope keys are allowed.
	envelope_keys_extra="$(jq -r '
		["ok", "outcome", "identity", "dry_run", "data", "meta", "error", "_notice"] as $allowed |
		if type == "object" then
			keys_unsorted[] | select(. as $k | $allowed | index($k) | not)
		else empty end' <"$envelope_keys_out")" || envelope_keys_extra=""
	if [ -n "$envelope_keys_extra" ]; then
		envelope_keys_extra_msg="non-envelope top-level key(s): $(printf '%s' "$envelope_keys_extra" | sort -u | tr '\n' ' ')"
		envelope_keys_violations="${envelope_keys_violations:+$envelope_keys_violations; }$envelope_keys_extra_msg"
	fi

	# Historical forms and every camelCase key are scanned recursively on the
	# meta/error structure. data is business payload and is intentionally out of
	# scope even when nested fields use camelCase or legacy names.
	envelope_keys_struct="$(jq -r '
		["success", "errcode", "err_code", "errCode", "error_code", "errorCode", "isSuccess", "is_success"] as $banned |
		select(type == "object") |
		["meta", "error"][] as $root |
		select(has($root)) |
		.[$root] | paths as $path |
		select(($path[-1] | type) == "string") |
		select(($path[-1] as $key | ($banned | index($key)) != null) or ($path[-1] | test("[a-z0-9][A-Z]"))) |
		([$root] + $path | map(tostring) | join("."))' <"$envelope_keys_out")" || envelope_keys_struct=""
	if [ -n "$envelope_keys_struct" ]; then
		envelope_keys_struct_msg="historical/retired key form(s) in envelope structure: $(printf '%s' "$envelope_keys_struct" | sort -u | tr '\n' ' ')"
		envelope_keys_violations="${envelope_keys_violations:+$envelope_keys_violations; }$envelope_keys_struct_msg"
	fi

	unified_result_report_violations "$envelope_keys_label" "$envelope_keys_violations"
}

unified_result_init "$ROOT"
unified_result_parse_args "$@"

self_test_cases() {
	printf 'envelope_legal_success|envelope|pass\n'
	printf 'envelope_legal_pending_dry_run|envelope|pass\n'
	printf 'envelope_legal_partial_failure|envelope|pass\n'
	printf 'envelope_legal_failure|envelope|pass\n'
	printf 'envelope_ok_full_meta|envelope|pass\n'
	printf 'envelope_ok_with_legacy_payload|envelope|pass\n'
	printf 'legacy_ok|legacy|pass\n'
	printf 'string_bool_ok|envelope|pass\n'
	printf 'envelope_legacy_keys|envelope|fail\n'
	printf 'envelope_camel_keys|envelope|fail\n'
	printf 'envelope_nested_camel_keys|envelope|fail\n'
	printf 'envelope_not_object|envelope|fail\n'
	printf 'envelope_missing_required|envelope|fail\n'
	printf 'envelope_invalid_outcome|envelope|fail\n'
	printf 'envelope_i1_mismatch|envelope|fail\n'
	printf 'envelope_i3_mismatch|envelope|fail\n'
	printf 'envelope_data_error|envelope|fail\n'
	printf 'envelope_disallowed_version_marker|envelope|fail\n'
	printf 'stdout_two_documents|envelope|fail\n'
	printf 'legacy_envelope_keys|envelope|fail\n'
	printf 'legacy_ok|envelope|fail\n'
	printf 'stdout_log_polluted|envelope|fail\n'
}

if [ "$SELF_TEST" -eq 1 ]; then
	unified_result_run_self_test envelope_keys_scan
fi

unified_result_scan_samples envelope_keys_scan
