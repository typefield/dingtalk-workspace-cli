# Agoal objectives / submit-detail Agent 边界审阅

扫描日期：2026-08-10

> 用户、规则、周期、模板和人员标识只在内存中使用；报告仅保存计数、字段名与机器契约结论，不保存原始 JSON，也不接入 CI / policy。

## Result: REVIEW

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 输入边界与精确 exclusion 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| 两条兼容命令 Help 可发现 | PASS | `rc=0` |
| 空周期/非法状态/日期/分页本地 fail-closed | PASS | `5/5 typed validation rc=3` |
| 目标读取前置事实可建立 | PASS | `user=yes, rule=yes, periods=18` |
| 已知规则周期的真实目标读取 | PASS | `periods=18, rows=0` |
| 提交详情前置 templateId 可建立 | PASS | `template=yes` |
| 显式日期三状态真实提交详情 | PASS | `ON_TIME/LATE/NOT_SUBMITTED containers reviewed` |
| 统计计数与显式日期详情计数一致 | REVIEW | `statistics={'ON_TIME': 0, 'LATE': 0, 'NOT_SUBMITTED': 0}, detail={'ON_TIME': 0, 'LATE': 0, 'NOT_SUBMITTED': 1}` |

## 非空字段证据

- `user objectives`: UNVERIFIED（本次没有非空目标行）
- `submit-detail ON_TIME`: rows=0; row_keys=UNVERIFIED; user_keys=UNVERIFIED
- `submit-detail LATE`: rows=0; row_keys=UNVERIFIED; user_keys=UNVERIFIED
- `submit-detail NOT_SUBMITTED`: rows=1; row_keys=publishTime,reportId,user; user_keys=dingUserId,id,name,workNo

## 结论

- `period-ids` 必须至少包含一个非空 ID；submit state、ISO 日期和显式页码均在任何业务调用前校验，失败为 `validation/invalid_flag_value`、rc=3。
- 空目标或空提交行只证明本次明确身份、规则、周期、模板、日期与状态范围没有返回记录，不能扩大为组织业务不存在。
- `list-statistics` 与显式日期 `submit-detail` 的三类计数必须分别记录；二者不一致时保持业务口径未知，不能用统计数覆盖详情或反向推断。
- 缺少非空稳定字段或三状态覆盖时，两条命令继续保持精确 exclusion；不将 legacy passthrough 包装成统一结果。

## Findings

- list-statistics and explicit-date submit-detail counts disagree
