# 统一结果表面 Agent 审计

扫描日期：2026-08-09

> 本报告由 Agent 在当前源码构建出的临时二进制上执行。它不是 CI / policy 门禁，只保存 Markdown 证据；所有 JSON 样本都只在临时目录存在。

## 结果摘要

| 项目 | 结果 |
|---|---|
| 临时构建当前二进制 | PASS |
| check-stdout-json.sh self-test | PASS |
| check-stdout-json.sh offline surface | PASS |
| check-string-bool.sh self-test | PASS |
| check-string-bool.sh offline surface | PASS |
| check-envelope-keys.sh self-test | PASS |
| check-envelope-keys.sh offline surface | PASS |

## 原始 Agent 证据

### 临时构建当前二进制

退出码：`0`

```text
(no stdout/stderr)
```

### check-stdout-json.sh self-test

退出码：`0`

```text
self-test ok: harness scans nonzero stdout and skips unavailable empty output
self-test ok: envelope_legal_success (pass)
self-test ok: envelope_legal_pending_dry_run (pass)
self-test ok: envelope_legal_partial_failure (pass)
self-test ok: envelope_legal_failure (pass)
self-test ok: envelope_ok_full_meta (pass)
self-test ok: stdout_polluted_info (fail)
self-test ok: stdout_log_polluted (fail)
self-test ok: stdout_two_documents (fail)
self-test ok: stdout_ansi_escape (fail)
self-test ok: stdout_empty (fail)
stdout-json self-test: ok
```

### check-stdout-json.sh offline surface

退出码：`0`

```text
stdout-json check: ok (scope=dev, verified=3/3, skipped=0)
```

### check-string-bool.sh self-test

退出码：`0`

```text
self-test ok: harness scans nonzero stdout and skips unavailable empty output
self-test ok: envelope_legal_success (pass)
self-test ok: envelope_legal_pending_dry_run (pass)
self-test ok: envelope_legal_partial_failure (pass)
self-test ok: envelope_legal_failure (pass)
self-test ok: envelope_ok_full_meta (pass)
self-test ok: envelope_ok_with_legacy_payload (pass)
self-test ok: string_bool_ok (pass)
self-test ok: legacy_ok (pass)
self-test ok: legacy_envelope_keys (pass)
self-test ok: string_bool_violation (fail)
self-test ok: stdout_log_polluted (fail)
string-bool self-test: ok
```

### check-string-bool.sh offline surface

退出码：`0`

```text
string-bool check: ok (scope=dev, verified=3/3, skipped=0)
```

### check-envelope-keys.sh self-test

退出码：`0`

```text
self-test ok: harness scans nonzero stdout and skips unavailable empty output
self-test ok: envelope_legal_success (pass)
self-test ok: envelope_legal_pending_dry_run (pass)
self-test ok: envelope_legal_partial_failure (pass)
self-test ok: envelope_legal_failure (pass)
self-test ok: envelope_ok_full_meta (pass)
self-test ok: envelope_ok_with_legacy_payload (pass)
self-test ok: legacy_ok (pass)
self-test ok: string_bool_ok (pass)
self-test ok: envelope_legacy_keys (fail)
self-test ok: envelope_camel_keys (fail)
self-test ok: envelope_nested_camel_keys (fail)
self-test ok: envelope_not_object (fail)
self-test ok: envelope_missing_required (fail)
self-test ok: envelope_invalid_outcome (fail)
self-test ok: envelope_i1_mismatch (fail)
self-test ok: envelope_i3_mismatch (fail)
self-test ok: envelope_data_error (fail)
self-test ok: envelope_disallowed_version_marker (fail)
self-test ok: stdout_two_documents (fail)
self-test ok: legacy_envelope_keys (fail)
self-test ok: legacy_ok (fail)
self-test ok: stdout_log_polluted (fail)
envelope-keys self-test: ok
```

### check-envelope-keys.sh offline surface

退出码：`0`

```text
envelope-keys check: ok (scope=dev, verified=3/3, skipped=0)
```

## 审阅边界

- 默认只运行离线、无登录、无网络、无副作用的 `dev` 样本；真实账号命令必须另行人工授权和取证。
- `--self-test` 中的 `contract_version` 仅是预期被拒绝的临时负向样本，不代表 CLI 输出或保存的结果。
- 本报告通过也只说明所列样本的 stdout 形状、布尔类型和顶层键符合契约；不能证明服务端终态、分页覆盖率或写入零副作用。
- 检查器不得接入 `make policy` 或 CI；需要新证据时由 Agent 重跑本脚本。
