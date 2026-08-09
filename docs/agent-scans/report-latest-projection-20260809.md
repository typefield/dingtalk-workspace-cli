# report +report-latest 投影 — Agent review

扫描时间：2026-08-09T17:49:45+08:00

> 本扫描只验证本地投影和失败分类，生成 Markdown 证据；不是 CI gate，不保存服务端响应或 JSON fixture。

| 检查 | 结果 |
|---|---|
| 未知列表容器返回 projection_unknown | PASS |
| 显式空数组与未知响应分离 | PASS |
| 非对象条目 fail-closed | PASS |
| 缺 reportId 不再输出原始行成功 | PASS |
| 部分时间证据不猜最新项 | PASS |
| 回归覆盖空/未知/非法/稳定选择 | PASS |
| 焦点 Go 回归 | PASS |

结论：**PASS**。

`report +report-latest` 仍保留在 Agent exclusion：它与规范的 outbox list + detail 路径重叠，且没有真实账号的稳定详情样本。本轮只关闭“未知响应伪装暂无日志”和“缺 reportId 原始行伪装成功”两项本地投影错误，不将其扩大为公共能力或真实终态证明。

```text
=== RUN   TestReportLatestProjectionDistinguishesEmptyFromUnknown
=== RUN   TestReportLatestProjectionDistinguishesEmptyFromUnknown/missing_stable_id
=== RUN   TestReportLatestProjectionDistinguishesEmptyFromUnknown/partial_time_coverage
=== RUN   TestReportLatestProjectionDistinguishesEmptyFromUnknown/unknown_container
=== RUN   TestReportLatestProjectionDistinguishesEmptyFromUnknown/wrong_container_type
=== RUN   TestReportLatestProjectionDistinguishesEmptyFromUnknown/non-object_row
--- PASS: TestReportLatestProjectionDistinguishesEmptyFromUnknown (0.00s)
    --- PASS: TestReportLatestProjectionDistinguishesEmptyFromUnknown/missing_stable_id (0.00s)
    --- PASS: TestReportLatestProjectionDistinguishesEmptyFromUnknown/partial_time_coverage (0.00s)
    --- PASS: TestReportLatestProjectionDistinguishesEmptyFromUnknown/unknown_container (0.00s)
    --- PASS: TestReportLatestProjectionDistinguishesEmptyFromUnknown/wrong_container_type (0.00s)
    --- PASS: TestReportLatestProjectionDistinguishesEmptyFromUnknown/non-object_row (0.00s)
=== RUN   TestReportLatestProjectionSelectsNewestStableEntry
--- PASS: TestReportLatestProjectionSelectsNewestStableEntry (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/smart	0.342s
```
