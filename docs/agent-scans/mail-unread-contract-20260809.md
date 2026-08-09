# 未读邮件聚合结果契约 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实邮箱索引或消息覆盖。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-mail: ORG 邮箱与消息成功投影 | PASS | rc=0 |
| mono-mail: 已知空列表才返回 count=0 | PASS | rc=0 |
| mono-mail: 邮箱发现 typed failure 原样分类 | PASS | rc=1 |
| mono-mail: 邮箱未知形状不伪装无邮箱 | PASS | rc=1 |
| mono-mail: 搜索失败不伪装空收件箱 | PASS | rc=1 |
| mono-mail: 搜索未知形状 fail-closed | PASS | rc=1 |
| mono-mail: dry-run 零 child 进程 | PASS | rc=0 |
| mono-mail: 非法 limit 写前/读前校验 | PASS | rc=1 |
| multi-mail: ORG 邮箱与消息成功投影 | PASS | rc=0 |
| multi-mail: 已知空列表才返回 count=0 | PASS | rc=0 |
| multi-mail: 邮箱发现 typed failure 原样分类 | PASS | rc=1 |
| multi-mail: 邮箱未知形状不伪装无邮箱 | PASS | rc=1 |
| multi-mail: 搜索失败不伪装空收件箱 | PASS | rc=1 |
| multi-mail: 搜索未知形状 fail-closed | PASS | rc=1 |
| multi-mail: dry-run 零 child 进程 | PASS | rc=0 |
| multi-mail: 非法 limit 写前/读前校验 | PASS | rc=1 |

结论：**16/16 PASS**。

范围：证明邮箱发现、搜索、投影、已知空、meta、dry-run 与参数校验的本地编排；真实邮箱权限、索引完整性和服务端终态仍需 live evidence。
