# Minutes 最近听记目标选择 — Agent review

扫描时间：2026-08-09T17:52:41+08:00

> 本扫描由 Agent 在当前工作树执行，只运行内存测试并输出 Markdown；不是 CI / policy gate，不保存任何服务端响应或 JSON fixture。

## Result: PASS

- `latestMinutesUUID` 拒绝通用 `id` 回退：**yes**
- 部分条目缺少时间时拒绝猜测最新项：**yes**
- 非法时间值 fail-closed：**yes**
- 焦点测试：`TestLatestMinutesTaskUUID`
- 测试退出码：`0`

## Required behavior

1. 最近听记复合读取只可使用 `taskUuid`、`taskUUID` 或 `uuid` 作为下游详情接口的 task UUID。
2. 仅有通用 `id` 的条目必须成为不可重试的 `api/projection_unknown`，不能被选择后继续读取另一个资源。
3. 明确空列表仍表示“暂无妙记”；未知容器、非法行或无真实 task UUID 均 fail-closed。
4. 只在所有条目都有可比较时间时按时间取最大值；全部无时间才采用服务端顺序，部分时间或非法时间必须 fail-closed。
5. `+latest-minutes`、`+action-items` 与 `+transcript` 继续保持隐藏；本修复不扩大 Agent 的公开命令面。

## Focused test transcript

```text
=== RUN   TestLatestMinutesTaskUUIDHandlesResultItemListAndKnownEmpty
--- PASS: TestLatestMinutesTaskUUIDHandlesResultItemListAndKnownEmpty (0.00s)
=== RUN   TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse
=== RUN   TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/generic_id_only
=== RUN   TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/partial_time_coverage
=== RUN   TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/invalid_time
=== RUN   TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/unknown_container
=== RUN   TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/malformed_row
=== RUN   TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/missing_task_uuid
--- PASS: TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse (0.00s)
    --- PASS: TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/generic_id_only (0.00s)
    --- PASS: TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/partial_time_coverage (0.00s)
    --- PASS: TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/invalid_time (0.00s)
    --- PASS: TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/unknown_container (0.00s)
    --- PASS: TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/malformed_row (0.00s)
    --- PASS: TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse/missing_task_uuid (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/smart	0.426s
```

## Boundary

这证明本地选择器不会将 Minutes 文档 ID 误作 task UUID。真实服务端不同响应形状、时间排序和后续详情读取仍须在隔离账号中由 Agent 单独取证。
