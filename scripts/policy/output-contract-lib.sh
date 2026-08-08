#!/bin/sh

# Shared library for the Phase I unified-output-contract CI scan prototypes
# (B161~B167): check-stdout-json.sh / check-string-bool.sh /
# check-envelope-keys.sh. Sourced by the three check scripts; not executable
# and not a policy gate by itself.
#
# Positioning: Phase I prototypes. They are NOT wired into `make policy`; the
# wiring design draft lives in scripts/policy/README.md (B167).
#
# Contract anchors:
#   - AC-02  ok/success-style booleans are always JSON booleans; string
#            booleans ("true"/"false") are violations.
#   - AC-11  json-mode stdout carries zero log bytes and exactly one primary
#            result document for success, pending, partial, or failure.
#   - Envelope top-level key set is fixed: ok/outcome/identity/dry_run/data/
#            meta/error/_notice (snake_case); historical variants such as
#            errcode/error_code/errorCode/success are violations.
#
# Scan scope (--scope, B166): dev (default) covers only dev-domain commands
# that are deterministic offline (isolated fresh HOME, no login, no network,
# no side effects). all additionally covers auth-dependent dev-domain reads
# and legacy non-envelope json commands (legacy class is exempt from the
# envelope-shape checks; see README.md).

output_contract_init() {
	OC_ROOT="$1"
	OC_BIN="${DWS_BIN:-$OC_ROOT/dws}"
	SCOPE="dev"
	SELF_TEST=0
	OC_FAILURES=0
}

output_contract_parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
		--scope)
			if [ $# -lt 2 ]; then
				output_contract_die_usage "--scope requires a value (dev|all)"
			fi
			shift
			SCOPE="$1"
			;;
		--scope=*)
			SCOPE="${1#--scope=}"
			;;
		--self-test)
			SELF_TEST=1
			;;
		-h | --help)
			output_contract_usage
			exit 0
			;;
		*)
			output_contract_die_usage "unknown argument: $1"
			;;
		esac
		shift
	done
	case "$SCOPE" in
	dev | all) ;;
	*)
		output_contract_die_usage "invalid --scope: $SCOPE (expected dev|all)"
		;;
	esac
}

output_contract_die_usage() {
	printf 'error: %s\n' "$1" >&2
	output_contract_usage >&2
	exit 2
}

# Samples: <class>\t<label>\t<argv>
# class=envelope: output must be a contract envelope (all checks apply).
# class=legacy:   pre-migration non-envelope json output; parseability and
#                 string-bool checks apply, envelope-shape checks do not.
# The dev-domain probe id is a throwaway identifier: no real connector/daemon
# can match it, so status/stop are side-effect-free no-ops.
output_contract_samples() {
	printf 'envelope\tdev-connect-list\tdev connect list --format json\n'
	printf 'envelope\tdev-connect-status\tdev connect status --unified-app-id dws-policy-scan-probe --format json\n'
	printf 'envelope\tdev-connect-stop-preview\tdev connect stop --unified-app-id dws-policy-scan-probe --dry-run --format json\n'
	if [ "$SCOPE" = "all" ]; then
		printf 'envelope\tdev-app-list\tdev app list --format json\n'
		printf 'envelope\tdevapp-shortcut-list\tdevapp +list --format json\n'
		printf 'legacy\tschema-list\tschema list -f json\n'
		printf 'legacy\tauth-status\tauth status --format json\n'
		printf 'legacy\tversion\tversion --format json\n'
	fi
}

# A unified failure or partial result intentionally exits nonzero but still owns one
# JSON envelope on stdout. Only a nonzero command with no stdout is unavailable
# to these scanners (for example, failure before output initialization).
output_contract_should_scan_result() {
	result_rc="$1"
	result_out="$2"
	if [ "$result_rc" -eq 0 ] || [ -s "$result_out" ]; then
		return 0
	fi
	return 1
}

output_contract_self_test_harness() {
	harness_test_tmp="$1"
	harness_test_fail=0
	: >"$harness_test_tmp/no-stdout"
	output_contract_materialize_fixture envelope_legal_partial_failure "$harness_test_tmp/partial.out"
	if ! output_contract_should_scan_result 7 "$harness_test_tmp/partial.out"; then
		printf 'self-test MISMATCH: nonzero unified envelope was skipped\n' >&2
		harness_test_fail=1
	fi
	if output_contract_should_scan_result 1 "$harness_test_tmp/no-stdout"; then
		printf 'self-test MISMATCH: empty nonzero result was not skipped\n' >&2
		harness_test_fail=1
	fi
	if [ "$harness_test_fail" -eq 0 ]; then
		printf 'self-test ok: harness scans nonzero stdout and skips unavailable empty output\n'
	fi
	return "$harness_test_fail"
}

# Materialize self-test inputs under a temporary directory. Keeping the
# fixtures executable here avoids a second, committed JSON authority while
# preserving the scanners' real file/stream behavior.
output_contract_materialize_fixture() {
	fixture_name="$1"
	fixture_path="$2"
	case "$fixture_name" in
	envelope_legal_success)
		jq -n '{ok:true,outcome:"success",data:[],meta:{count:0}}' >"$fixture_path"
		;;
	envelope_legal_pending_dry_run)
		jq -n '{
			ok:true,outcome:"pending",dry_run:true,
			data:{
				invocation:{kind:"helper_invocation",stage:"helper_override",implemented:false,dry_run:true,canonical_product:"devapp",tool:"create_dev_app",canonical_path:"devapp.create_dev_app",legacy_path:"dev app create",params:{desc:"probe",name:"ProbeApp"}},
				response:{dry_run:true,note:"execution skipped by --dry-run",request:{id:3,jsonrpc:"2.0",method:"tools/call",params:{arguments:{desc:"probe",name:"ProbeApp"},name:"create_dev_app"}}}
			}
		}' >"$fixture_path"
		;;
	envelope_legal_partial_failure)
		jq -n '{ok:false,outcome:"partial_failure",data:{succeeded:[{id:"one"}],failed:[{id:"two",error:{type:"remote"}}],unknown:[]}}' >"$fixture_path"
		;;
	envelope_legal_failure)
		jq -n '{ok:false,outcome:"failure",error:{type:"unauthorized",message:"fixture",retryable:false}}' >"$fixture_path"
		;;
	envelope_ok_full_meta)
		jq -n '{ok:true,outcome:"success",identity:"user:scan-fixture",data:{items:[{name:"fixture",unifiedAppId:"00000000-0000-0000-0000-000000000000"}]},meta:{count:1,pagination:{endpoint_exhausted:false,next_token:"fixture-token"}}}' >"$fixture_path"
		;;
	envelope_ok_with_legacy_payload)
		jq -n '{ok:true,outcome:"success",data:{hasMore:false,items:[{success:"true",errorCode:"BUSINESS_FIELD_NOT_ENVELOPE",timedOut:false,name:"business payload is not envelope structure"}]}}' >"$fixture_path"
		;;
	legacy_ok)
		jq -n '{success:true,authenticated:false,message:"legacy non-envelope shape (pre-migration json output)"}' >"$fixture_path"
		;;
	string_bool_ok)
		jq -n '{ok:true,outcome:"success",data:[],meta:{count:0,pagination:{endpoint_exhausted:false,next_token:"fixture-next"}}}' >"$fixture_path"
		;;
	envelope_legacy_keys)
		jq -n '{ok:true,outcome:"success",success:true,error_code:"",data:{}}' >"$fixture_path"
		;;
	envelope_camel_keys)
		jq -n '{ok:true,outcome:"pending",meta:{timedOut:false,nextCommand:"dws dev app robot result --unified-app-id fixture"},data:{}}' >"$fixture_path"
		;;
	envelope_nested_camel_keys)
		jq -n '{ok:false,outcome:"failure",error:{type:"remote",message:"fixture",details:{retryAfter:3}},meta:{operation:{progress:[{nextCommand:"dws fixture result"}]}}}' >"$fixture_path"
		;;
	envelope_not_object)
		jq -n '[]' >"$fixture_path"
		;;
	envelope_missing_required)
		jq -n '{data:{}}' >"$fixture_path"
		;;
	envelope_invalid_outcome)
		jq -n '{ok:false,outcome:"partial"}' >"$fixture_path"
		;;
	envelope_i1_mismatch)
		jq -n '{ok:true,outcome:"partial_failure",data:{}}' >"$fixture_path"
		;;
	envelope_i3_mismatch)
		jq -n '{ok:false,outcome:"failure"}' >"$fixture_path"
		;;
	envelope_data_error)
		jq -n '{ok:false,outcome:"failure",data:{},error:{type:"fixture",message:"fixture"}}' >"$fixture_path"
		;;
	envelope_contract_version)
		jq -n '{ok:true,outcome:"success",contract_version:"2",data:{}}' >"$fixture_path"
		;;
	legacy_envelope_keys)
		jq -n '{ok:true,outcome:"success",errcode:0,errorCode:"0",error_code:"0",success:false,data:{timedOut:false,nextToken:"n"}}' >"$fixture_path"
		;;
	string_bool_violation)
		jq -n '{ok:"true",outcome:"success",data:{},meta:{supervised:"false"}}' >"$fixture_path"
		;;
	stdout_polluted_info)
		{
			printf '[INFO] discovered 3 MCP services\n'
			jq -n '{ok:true,outcome:"success",data:{}}'
		} >"$fixture_path"
		;;
	stdout_log_polluted)
		{
			printf '[INFO] 正在加载连接器记录...\n'
			jq -n '{ok:true,outcome:"success",data:[],meta:{count:0}}'
			printf '[WARN] cache miss\n'
		} >"$fixture_path"
		;;
	stdout_two_documents)
		{
			jq -c -n '{ok:true,outcome:"success",data:{}}'
			jq -c -n '{ok:true,outcome:"success",data:{}}'
		} >"$fixture_path"
		;;
	stdout_ansi_escape)
		printf '{"ok":true,"outcome":"success","data":"colored \033[31mtext\033[0m leaked into json stdout"}\n' >"$fixture_path"
		;;
	stdout_empty)
		: >"$fixture_path"
		;;
	*)
		printf 'unknown output-contract self-test fixture: %s\n' "$fixture_name" >&2
		return 1
		;;
	esac
}

output_contract_require_jq() {
	if ! command -v jq >/dev/null 2>&1; then
		printf 'error: jq is required by the output-contract scan prototypes\n' >&2
		exit 2
	fi
}

# output_contract_run_self_test <scan_fn>
# <scan_fn> <class> <label> <file> prints violation lines (empty output means
# pass). Fixtures and expectations come from the caller-defined
# self_test_cases() emitting "<fixture>|<class>|pass" / "<fixture>|<class>|fail"
# lines (class is passed through so legacy-exemption rules are testable).
output_contract_run_self_test() {
	self_test_scan_fn="$1"
	output_contract_require_jq
	self_test_tmp="$(mktemp -d)"
	trap 'rm -rf "$self_test_tmp"' EXIT HUP INT TERM
	self_test_cases >"$self_test_tmp/cases"
	self_test_fail=0
	if ! output_contract_self_test_harness "$self_test_tmp"; then
		self_test_fail=1
	fi
	while IFS='|' read -r self_test_fixture self_test_class self_test_expect; do
		if [ -z "$self_test_fixture" ]; then
			continue
		fi
		self_test_path="$self_test_tmp/$self_test_fixture.out"
		if ! output_contract_materialize_fixture "$self_test_fixture" "$self_test_path"; then
			printf 'self-test: failed to materialize fixture %s\n' "$self_test_fixture" >&2
			self_test_fail=1
			continue
		fi
		self_test_violations="$("$self_test_scan_fn" "$self_test_class" "$self_test_fixture" "$self_test_path")" || true
		if [ -n "$self_test_violations" ]; then
			self_test_got="fail"
		else
			self_test_got="pass"
		fi
		if [ "$self_test_got" != "$self_test_expect" ]; then
			self_test_fail=1
			printf 'self-test MISMATCH: %s expected=%s got=%s\n' \
				"$self_test_fixture" "$self_test_expect" "$self_test_got" >&2
			if [ -n "$self_test_violations" ]; then
				printf '%s\n' "$self_test_violations" >&2
			fi
		else
			printf 'self-test ok: %s (%s)\n' "$self_test_fixture" "$self_test_expect"
		fi
	done <"$self_test_tmp/cases"
	if [ "$self_test_fail" -ne 0 ]; then
		printf '%s self-test: FAILED\n' "$OC_SCRIPT_NAME" >&2
		exit 1
	fi
	printf '%s self-test: ok\n' "$OC_SCRIPT_NAME"
	exit 0
}

# output_contract_scan_samples <process_fn>
# <process_fn> <class> <label> <stdout_file> <stderr_file> inspects one
# captured sample and increments OC_FAILURES on violations.
output_contract_scan_samples() {
	scan_process_fn="$1"
	output_contract_require_jq
	if [ ! -x "$OC_BIN" ]; then
		printf 'error: dws binary not found at %s (run make build first)\n' "$OC_BIN" >&2
		exit 2
	fi
	scan_tmp="$(mktemp -d)"
	trap 'rm -rf "$scan_tmp"' EXIT HUP INT TERM
	# HOME selection:
	#   - DWS_SCAN_HOME override wins (operator-controlled environment).
	#   - scope=dev (default): isolated fresh HOME + DWS_DISABLE_KEYCHAIN=1 so
	#     the scan is deterministic, login-free, and side-effect-free.
	#   - scope=all: inherits the real HOME because auth-dependent samples
	#     (dev app list) need a logged-in session.
	scan_disable_keychain=0
	if [ -n "${DWS_SCAN_HOME:-}" ]; then
		scan_home="$DWS_SCAN_HOME"
	elif [ "$SCOPE" = "dev" ]; then
		scan_home="$(mktemp -d "$scan_tmp/scan-home.XXXXXX")"
		scan_disable_keychain=1
	else
		scan_home="${HOME:-$scan_tmp/home}"
	fi
	scan_samples="$scan_tmp/samples"
	output_contract_samples >"$scan_samples"
	scan_tab="$(printf '\t')"
	scan_total=0
	scan_verified=0
	scan_skipped=0
	while IFS="$scan_tab" read -r scan_class scan_label scan_argv; do
		if [ -z "$scan_label" ]; then
			continue
		fi
		scan_total=$((scan_total + 1))
		scan_out="$scan_tmp/$scan_label.stdout"
		scan_err="$scan_tmp/$scan_label.stderr"
		scan_rc=0
		scan_attempt=1
		# Hard discipline: transient in-flight edits of the shared binary can
		# cause sporadic failures; retry once before drawing conclusions.
		while [ "$scan_attempt" -le 2 ]; do
			scan_rc=0
			if [ "$scan_disable_keychain" -eq 1 ]; then
				HOME="$scan_home" DWS_DISABLE_KEYCHAIN=1 "$OC_BIN" $scan_argv \
					>"$scan_out" 2>"$scan_err" </dev/null || scan_rc=$?
			else
				HOME="$scan_home" "$OC_BIN" $scan_argv \
					>"$scan_out" 2>"$scan_err" </dev/null || scan_rc=$?
			fi
			# A nonzero unified result is still a completed sample. Preserve its first
			# envelope instead of retrying and potentially overwriting it.
			if [ "$scan_rc" -eq 0 ] || [ -s "$scan_out" ]; then
				break
			fi
			scan_attempt=$((scan_attempt + 1))
		done
		if ! output_contract_should_scan_result "$scan_rc" "$scan_out"; then
			scan_skipped=$((scan_skipped + 1))
			printf '  [skip] %s: exited rc=%s with no stdout after retry; stderr tail: %s\n' \
				"$scan_label" "$scan_rc" "$(tail -c 160 "$scan_err" 2>/dev/null | tr '\n' ' ')"
			continue
		fi
		if [ "$scan_rc" -ne 0 ]; then
			printf '  [scan] %s: exited rc=%s with stdout; validating expected unified failure/partial output\n' \
				"$scan_label" "$scan_rc"
		fi
		scan_verified=$((scan_verified + 1))
		"$scan_process_fn" "$scan_class" "$scan_label" "$scan_out" "$scan_err"
	done <"$scan_samples"
	if [ "$scan_verified" -eq 0 ]; then
		printf 'error: %s verified no sample successfully (scope=%s); refusing to pass vacuously\n' \
			"$OC_SCRIPT_NAME" "$SCOPE" >&2
		exit 1
	fi
	if [ "$OC_FAILURES" -gt 0 ]; then
		printf '%s check: FAILED (%s violation(s), scope=%s, verified=%s/%s, skipped=%s)\n' \
			"$OC_SCRIPT_NAME" "$OC_FAILURES" "$SCOPE" "$scan_verified" "$scan_total" "$scan_skipped" >&2
		exit 1
	fi
	printf '%s check: ok (scope=%s, verified=%s/%s, skipped=%s)\n' \
		"$OC_SCRIPT_NAME" "$SCOPE" "$scan_verified" "$scan_total" "$scan_skipped"
}

# output_contract_report_violations <label> <violations>
output_contract_report_violations() {
	report_label="$1"
	report_violations="$2"
	if [ -z "$report_violations" ]; then
		return 0
	fi
	OC_FAILURES=$((OC_FAILURES + 1))
	printf '%s\n' "$report_violations" | while IFS= read -r report_line; do
		printf '  [%s] %s\n' "$report_label" "$report_line"
	done
}
