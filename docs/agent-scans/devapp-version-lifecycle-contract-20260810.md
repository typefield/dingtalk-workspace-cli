# DevApp 版本生命周期结果与渐进 rollout Agent 扫描

扫描日期：2026-08-10

> 本报告由 Agent 在当前工作树执行；仅使用 dry-run 和受控 caller，不创建/发布真实版本、不保存 JSON fixture，也不接入 CI。

| 检查 | 结果 | 脱敏证据 |
|---|---|---|
| 两条版本写 Shortcut 保持逐命令 dual_validate | PASS | 没有用户选择协议；真实写响应与回读证据齐全前，外部仍保持 legacy、内部 shadow 校验统一结果。 |
| 两套入口发布同源版本结果契约 | PASS | create/check-approval/publish/status 的 outcome 与 data schema 在 dev、devapp 两入口同源。 |
| 创建版本要求稳定 versionId 且不伪造回读 | PASS | 只有稳定 versionId 才能成功；请求事实在 requested，下游继续使用明确的 status 回读命令。 |
| 发布区分终态、审批 pending 与未知效果 | PASS | 审批提交/审核中为 pending；只有显式 RELEASE/GRAY/published 才进入 success，且仍标 not_verified。 |
| 冲突状态 fail-closed 并保留恢复路径 | PASS | processStatus 失败优先于笼统 SUCCESS；pending 可从原请求补齐 versionId/unifiedAppId 和 next_command。 |
| 版本写 Safety 与 guard-first 对齐 | PASS | create/publish 均 write/high、user_required、idempotency unknown，确认发生在业务参数与调用之前。 |
| 受控版本状态与 Schema 矩阵 | PASS | 受控创建/发布对象、审批 pending、冲突状态、未知 ACK、一次调用与最终 Schema 测试通过。 |
| 公开 CLI 渐进输出与零写边界 | PASS | 四条 dry-run 均零写：native 为统一信封、dual Shortcut 保持 legacy；八条 live Schema 的 Safety/ResultSpec 同源且无版本标记。 |

结论：**8/8 PASS**。版本创建不再接受无 `versionId` 的模糊 ACK；发布明确区分审批 pending、响应声称终态和未知效果。

边界：当前没有隔离应用的真实创建/发布与回读证据。两条写 Shortcut 继续 `dual_validate` 且保持 exclusion；只有真实响应、审批链和 `version status/get` 回读闭环后才允许逐条晋级或公开。
