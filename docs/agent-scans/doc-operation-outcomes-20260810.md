# Doc 复合写 operation outcome Agent 扫描

扫描日期：2026-08-10

> Agent 在当前工作树执行扫描；只运行零写入 dry-run、确认前拦截和受控 response seam。报告不保存 JSON fixture，不调用真实写接口，也不接入 CI。

| 检查 | 结果 | 脱敏证据 |
|---|---|---|
| 三个 terminal command 独立 active | PASS | create/checkpoint-update/history-revert 由发布声明选定统一结果；Agent 不选择协议。 |
| 成功结果不嵌套 legacy 信封 | PASS | legacy payload 仅供旧阶段；active data 固定为 operation/result/steps。 |
| partial 使用三通道 | PASS | 已应用步骤、失败步骤和未开始步骤分别进入 succeeded/failed/unknown。 |
| active lifecycle 有命令级矩阵 | PASS | 三条命令分别覆盖 success、dry-run 和 partial，且检查调用次数。 |
| 受控 success/partial/调用次数矩阵 | PASS | 受控 response seam 全部通过；每个业务流程只执行一次。 |
| 公开 CLI 预览与确认边界 | PASS | create/checkpoint 零写预览 + history-revert 确认前拒绝；均为单一统一信封且无版本标记。 |

结论：**6/6 PASS**。三条 terminal command 现直接使用统一结果：成功/预览由框架生成 `ok/outcome/data`，复合写部分失败保留 `succeeded/failed/unknown` 并返回 rc=7；Agent 只传 `--format json`。

未验证：真实租户中创建后的 JSONML 写入失败、checkpoint 更新失败、回滚后读回失败及补偿动作的服务端终态。active 只保证已识别事实被准确表达，不把本地 fixture 扩大为远端事务证明。
