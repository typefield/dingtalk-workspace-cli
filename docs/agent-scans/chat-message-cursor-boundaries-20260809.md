# Chat 消息分页游标精度 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行此扫描；它只验证本地归一化逻辑和 fixture，不调用真实 DingTalk，也不保存 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| chat-messages 拒绝浮点 MaxInt64 精度边界 | PASS | 浮点 JSON number 在该边界会舍入，不能再转换为 int64 游标。 |
| thread-replies 拒绝浮点 MaxInt64 精度边界 | PASS | 话题回复与消息历史使用同一 fail-closed 规则。 |
| 终页零游标仍被识别为 endpoint exhausted | PASS | 修复异常浮点边界不能把 API 的 nextCursor=0 终态哨兵误判为失败。 |
| Agent 公开调用仍为 --format json | PASS | 验证不引入协议版本或专用 cursor 参数。 |
| fixture-backed precision/terminal semantics | PASS | rc=0 |

结论：**5/5 PASS**。消息历史与话题回复均拒绝浮点 JSON number 无法精确表示的 `MaxInt64` 游标，避免生成错误的续页时间；`nextCursor=0` 仍只表示 endpoint 已耗尽。

未验证：真实网关返回值的数值解码类型与服务端分页终态；这些仍需真实账号复验。
