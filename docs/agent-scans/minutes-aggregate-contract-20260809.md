# Minutes 聚合脚本结果契约 Agent 探针

扫描日期：2026-08-09

> 以临时假 dws 对拍 Mono/Multi 四个入口；不保存 JSON fixture，不证明真实听记内容或服务端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-summary: 全成功保留 child meta | PASS | rc=0 |
| mono-summary: 单项失败保留成功项 | PASS | rc=7 |
| mono-summary: 单项投影漂移不伪装空内容 | PASS | rc=7 |
| mono-summary: display-only id 列表 fail-closed | PASS | rc=1 |
| mono-summary: text 模式保持 partial rc=7 | PASS | rc=7 |
| mono-todos: 全成功保留 child meta | PASS | rc=0 |
| mono-todos: 单项失败保留成功项 | PASS | rc=7 |
| mono-todos: 单项投影漂移不伪装空内容 | PASS | rc=7 |
| mono-todos: display-only id 列表 fail-closed | PASS | rc=1 |
| mono-todos: text 模式保持 partial rc=7 | PASS | rc=7 |
| mono-todos: 指定 ID dry-run 零 child 调用 | PASS | rc=0 |
| multi-summary: 全成功保留 child meta | PASS | rc=0 |
| multi-summary: 单项失败保留成功项 | PASS | rc=7 |
| multi-summary: 单项投影漂移不伪装空内容 | PASS | rc=7 |
| multi-summary: display-only id 列表 fail-closed | PASS | rc=1 |
| multi-summary: text 模式保持 partial rc=7 | PASS | rc=7 |
| multi-todos: 全成功保留 child meta | PASS | rc=0 |
| multi-todos: 单项失败保留成功项 | PASS | rc=7 |
| multi-todos: 单项投影漂移不伪装空内容 | PASS | rc=7 |
| multi-todos: display-only id 列表 fail-closed | PASS | rc=1 |
| multi-todos: text 模式保持 partial rc=7 | PASS | rc=7 |
| multi-todos: 指定 ID dry-run 零 child 调用 | PASS | rc=0 |

结论：**22/22 PASS**。

范围：证明列表/逐项读取失败、投影漂移、partial/text 退出码、meta 与指定 ID dry-run 的本地编排；索引覆盖、分页耗尽和真实内容仍需 live evidence。
