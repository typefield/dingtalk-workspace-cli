# Drive 目录树分页结果契约 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实钉盘权限、目录规模或服务端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-drive: 根与子目录逐页耗尽 | PASS | rc=0, calls=3 |
| mono-drive: 已知空目录才返回成功空 | PASS | rc=0 |
| mono-drive: 首请求失败不伪装空目录 | PASS | rc=1 |
| mono-drive: 后续页失败保留首批数据 | PASS | rc=7 |
| mono-drive: 子目录失败保留根目录 | PASS | rc=7 |
| mono-drive: 未知列表形状 fail-closed | PASS | rc=1 |
| mono-drive: 缺稳定 fileId fail-closed | PASS | rc=1 |
| mono-drive: 缺续页游标保留当前页 | PASS | rc=7 |
| mono-drive: 重复游标停止且 partial | PASS | rc=7 |
| mono-drive: dry-run 零 child 进程 | PASS | rc=0, calls=0 |
| mono-drive: 旧 parent-id 仅作兼容别名 | PASS | rc=0 |
| mono-drive: 非法 depth 调用前校验 | PASS | rc=1 |
| mono-drive: text 与 JSON 使用同一次遍历结果 | PASS | rc=7, calls=2 |
| multi-drive: 根与子目录逐页耗尽 | PASS | rc=0, calls=3 |
| multi-drive: 已知空目录才返回成功空 | PASS | rc=0 |
| multi-drive: 首请求失败不伪装空目录 | PASS | rc=1 |
| multi-drive: 后续页失败保留首批数据 | PASS | rc=7 |
| multi-drive: 子目录失败保留根目录 | PASS | rc=7 |
| multi-drive: 未知列表形状 fail-closed | PASS | rc=1 |
| multi-drive: 缺稳定 fileId fail-closed | PASS | rc=1 |
| multi-drive: 缺续页游标保留当前页 | PASS | rc=7 |
| multi-drive: 重复游标停止且 partial | PASS | rc=7 |
| multi-drive: dry-run 零 child 进程 | PASS | rc=0, calls=0 |
| multi-drive: 旧 parent-id 仅作兼容别名 | PASS | rc=0 |
| multi-drive: 非法 depth 调用前校验 | PASS | rc=1 |
| multi-drive: text 与 JSON 使用同一次遍历结果 | PASS | rc=7, calls=2 |

结论：**26/26 PASS**。

范围：证明逐目录分页、单次遍历、已知空、投影 fail-closed、游标异常、部分失败、dry-run 和兼容别名；真实账号下的目录可见范围仍需 live evidence。
