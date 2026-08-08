# Multi 本地结果边界 Agent 语义对拍

各运行时仅在临时 Python 子进程中加载；本报告不保存 JSON fixture，不调用 dws，也不替代真实服务端终态验证。

| 检查 | 结果 | 证据 |
|---|---|---|
| AITable: 未捕获异常不泄漏 traceback | PASS | rc=1; single_envelope=True |
| AITable: 旧 success:false 不按 truthiness 成功 | PASS | rc=0; payload={'state': 'unknown', 'error_type': 'api'} |
| AITable: partial_failure 保留三通道并返回 rc=7 | PASS | rc=7; single_envelope=True |
| Todo: 未捕获异常不泄漏 traceback | PASS | rc=1; single_envelope=True |
| Todo: 旧 success:false 不按 truthiness 成功 | PASS | rc=0; payload={'state': 'unknown', 'error_type': 'api'} |
| Todo: partial_failure 保留三通道并返回 rc=7 | PASS | rc=7; single_envelope=True |
| Misc: 未捕获异常不泄漏 traceback | PASS | rc=1; single_envelope=True |
| Misc: 旧 success:false 不按 truthiness 成功 | PASS | rc=0; payload={'state': 'unknown', 'error_type': 'api'} |
| Misc: partial_failure 保留三通道并返回 rc=7 | PASS | rc=7; single_envelope=True |

结论：**9/9 PASS**。

范围：横向验证异常边界、历史字符串布尔失败分类和批量部分成功的机器契约；业务写入、分页和服务端终态仍由各产品的 Agent 探针与真实环境证据负责。
