# Event stop 统一结果契约 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行此扫描；它只验证声明、门禁和本地 lifecycle fixture，不调用真实 DingTalk，也不保存 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| 迁移状态为 unified_active | PASS | 协议由发布声明决定；Agent 仅传 --format json，不选择版本。 |
| dry-run 是完整 success 结果而非停机成功 | PASS | 预览经统一 lifecycle 输出 dry_run:true，确认门禁前不调用取消路径。 |
| 已确认取消与未知后续状态保留三通道 | PASS | 已取消订阅进入 succeeded[]；后续清理/控制面不确定进入 unknown[]，rc=7。 |
| 普通终态成功也走 StoreResult | PASS | 不会只统一失败、让成功继续手写 stdout。 |
| 结果声明覆盖 success/partial_failure/failure | PASS | Schema/Skill 发现层可获得真实终态集合。 |
| 本地 fixture 覆盖 active 的 preview、success 与 partial | PASS | 测试根挂载真实 ResultStore/PostRun，覆盖 preview、success 与 partial，避免把 direct Cobra 调用误当生产生命周期。 |
| fixture-backed active lifecycle | PASS | rc=0 |
| public CLI dry-run / confirmation boundary | PASS | dry-run success + no-confirm typed validation; no version marker |

结论：**8/8 PASS**。`event stop` 已处于 `unified_active`：缺确认会在写入前失败；dry-run 是可解析预览；确认取消后若后续阶段不可确认，已完成项不会丢失而是输出 `partial_failure`/rc=7。Agent 只使用 `--format json`，不选择协议版本。

未验证：真实订阅取消后的远端订阅、本地消费者进程和 run-state 三者是否最终一致；这仍需要隔离账号和受控进程环境复验，不能由本地 fixture 推断。
