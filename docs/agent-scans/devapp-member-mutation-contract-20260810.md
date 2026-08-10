# DevApp 成员写入结果与渐进 rollout Agent 扫描

扫描日期：2026-08-10

> 本报告由 Agent 在当前工作树执行；仅使用 dry-run 和受控 caller，不添加/移除真实成员、不保存 JSON fixture，也不接入 CI。

| 检查 | 结果 | 脱敏证据 |
|---|---|---|
| 两条旧 Shortcut 保持逐命令 dual_validate | PASS | 没有用户选择协议；真实 helper 响应取证前仍输出 legacy，shadow 构造统一结果。 |
| 两套入口共用成员写结果契约 | PASS | dev 与 devapp 均声明 success/pending/partial/failure 的同源 ResultSpec。 |
| 请求事实不冒充逐成员成功 | PASS | 成功保留上游对象，同时把 userIds 放在 requested 下并标 verification:not_verified。 |
| 投影未知 fail-closed | PASS | 非对象响应或请求身份丢失均为 projection_unknown，且禁止盲目重试。 |
| 安全等级两入口对齐 | PASS | 添加为 write/high；移除为 destructive/high；两者都 user_required、idempotency unknown。 |
| 受控结果与 Safety 矩阵 | PASS | 受控对象/非对象响应、一次调用、请求投影和最终 Schema Safety 测试通过。 |
| 公开 CLI 渐进输出与零写边界 | PASS | 四条公开 dry-run 均零写：native 为统一信封，dual Shortcut 逐字保留 legacy JSON；live Schema 的 Safety/ResultSpec 同源且无版本标记。 |

结论：**7/7 PASS**。两套入口已共用诚实的成员写结果模型；请求列表只作为 `requested` 事实，不能当作逐成员已生效证明。

边界：官方仓库和本地评测证据都没有 dingtalk-dev helper 的真实成功响应样本，因此两个既有 Shortcut 继续 `dual_validate`。取得脱敏真实对象响应并完成成员列表回读前，不晋级 active、不宣称逐成员终态。
