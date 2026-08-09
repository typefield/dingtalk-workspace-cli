# Agoal scorecard detail Agent 边界审阅

扫描日期：2026-08-10

> Agent 使用当前源码临时构建并做有界真实只读抽样；部门 ID、计分卡 ID、业务内容和原始 JSON 只在内存中处理，不写入本报告，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| null 拒绝、legacy 保真与 exclusion 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| 兼容命令 Help 保持可发现 | PASS | `rc=0, flags=yes` |
| 证据不足命令仍保持精确 exclusion | PASS | `public_agoal_tools=4` |
| 根部门非 null legacy 响应保持可用 | PASS | `entity_rows=0, stable_scorecard_id=yes` |
| 直属部门 null 不再伪装成功 | PASS | `sampled=10, null_rejected=8, empty_success=2, nonempty_success=0, rc0_null=0` |
| 非空核心实体投影证据 | PASS | `UNVERIFIED（抽样未观察到非空 content）` |

## 结论

- 旧行为会把服务端 JSON null 原样写到 stdout 并 rc=0；当前边界改为 typed `api/projection_unknown`、retryable=false、非零退出，且 stdout 为空。
- 非 null 响应继续使用 legacy writer，业务请求 exactly-once；本修复不借机改变输出结构或公开统一契约。
- 根部门及有界直属部门样本未观察到非空核心 `content` 实体，因此无法审阅 entityId、嵌套目标或 update 所需完整内容。
- `agoal scorecard detail` 继续保留精确 exclusion；取得非空实体样本并定义完整 ResultSpec 后再进入 dual validation。
