# Drive `+list` live projection — Agent evidence

扫描时间：2026-08-09T20:56:43+08:00

> 当前源码构建二进制执行只读目录列表。报告只保存计数、类型和分页状态；不保存文件名、dentry ID、cursor、原始 JSON 或账号信息。

## Result: PASS

| 检查项 | 结果 |
|---|---|
| 首屏统一信封与 scoped inventory | PASS |
| token-only continuation 可恢复 | PASS |
| 续页仍保持统一投影 | PASS |
| 无终态证据不伪造 endpoint exhausted | PASS |
| 结果不含协议版本或 legacy 分页键 | PASS |

## Redacted observations

- 首屏：count=1, stable_ids=True, scope=requested_location, continuation=True。
- 续页：count=1, stable_ids=True, scope=requested_location。
- 宽页：count=41, pagination_known=False, pagination_meta_present=False。

## Boundary

- 非空 token 只证明当前 endpoint 可续页；它不证明租户 Drive 全量目录、索引健康或权限覆盖。
- token 缺失且没有显式终态布尔时保持 `pagination_known:false`，不把当前页包装成 endpoint exhausted。
- 本证据不注入服务端异常，未知容器、非法条目、矛盾 `hasMore/token` 仍由本地 fixture 回归证明 fail-closed。
