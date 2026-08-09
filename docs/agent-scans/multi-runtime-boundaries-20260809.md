# Multi 结果边界 Agent 语义对拍

各运行时仅在临时 Python 子进程中加载；本报告不保存 JSON fixture，不调用 dws，也不替代真实服务端终态验证。

| 检查 | 结果 | 证据 |
|---|---|---|
| AITable: 未捕获异常不泄漏 traceback | PASS | rc=1; single_envelope=True |
| AITable: 旧 success:false 不按 truthiness 成功 | PASS | rc=0; payload={'state': 'unknown', 'error_type': 'api'} |
| AITable: success 字符串不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| AITable: 矛盾 ok/outcome 不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| AITable: 非字符串 outcome 不泄漏异常或伪装成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| AITable: pending 不伪装终态成功且保留任务 meta | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'operation_pending', 'meta': {'operation': {'id': 'task-1'}}} |
| AITable: partial_failure 保留三通道并返回 rc=7 | PASS | rc=7; single_envelope=True |
| Todo: 未捕获异常不泄漏 traceback | PASS | rc=1; single_envelope=True |
| Todo: 旧 success:false 不按 truthiness 成功 | PASS | rc=0; payload={'state': 'unknown', 'error_type': 'api'} |
| Todo: success 字符串不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Todo: 矛盾 ok/outcome 不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Todo: 非字符串 outcome 不泄漏异常或伪装成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Todo: pending 不伪装终态成功且保留任务 meta | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'operation_pending', 'meta': {'operation': {'id': 'task-1'}}} |
| Todo: partial_failure 保留三通道并返回 rc=7 | PASS | rc=7; single_envelope=True |
| Misc: 未捕获异常不泄漏 traceback | PASS | rc=1; single_envelope=True |
| Misc: 旧 success:false 不按 truthiness 成功 | PASS | rc=0; payload={'state': 'unknown', 'error_type': 'api'} |
| Misc: success 字符串不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Misc: 矛盾 ok/outcome 不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Misc: 非字符串 outcome 不泄漏异常或伪装成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Misc: pending 不伪装终态成功且保留任务 meta | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'operation_pending', 'meta': {'operation': {'id': 'task-1'}}} |
| Misc: partial_failure 保留三通道并返回 rc=7 | PASS | rc=7; single_envelope=True |
| Shared: 未捕获异常不泄漏 traceback | PASS | rc=1; single_envelope=True |
| Shared: 旧 success:false 不按 truthiness 成功 | PASS | rc=0; payload={'state': 'unknown', 'error_type': 'api'} |
| Shared: success 字符串不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Shared: 矛盾 ok/outcome 不伪装执行成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Shared: 非字符串 outcome 不泄漏异常或伪装成功 | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'untyped_status'} |
| Shared: pending 不伪装终态成功且保留任务 meta | PASS | rc=0; payload={'state': 'unknown', 'subtype': 'operation_pending', 'meta': {'operation': {'id': 'task-1'}}} |
| Shared: partial_failure 保留三通道并返回 rc=7 | PASS | rc=7; single_envelope=True |

结论：**28/28 PASS**。

范围：横向验证局部与 shared 运行时的异常边界、历史字符串布尔失败分类和 partial_failure/rc=7 机器契约；业务写入、分页和服务端终态仍由各产品的 Agent 探针与真实环境证据负责。
