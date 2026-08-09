# Agoal report submit-detail Agent 边界审阅

扫描日期：2026-08-10

> Agent 使用当前源码临时构建，仅执行真实只读请求。模板 ID、人员 ID、姓名、工号、请求 ID 和原始 JSON 只在临时文件与内存中处理，不写入本报告，也不接入 CI / policy。

## Result: 保持精确 exclusion

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| `ON_TIME` 响应容器 | PASS | `content{page,pageSize,result,totalCount}`；本次 `result=0` |
| `LATE` 响应容器 | PASS | `content{page,pageSize,result,totalCount}`；本次 `result=0` |
| `NOT_SUBMITTED` 非空行 | PASS | `result=1`；行含 `user/reportId/publishTime`，用户对象含稳定 ID |
| 三种状态的非空行投影覆盖 | REVIEW | 缺少非空 `ON_TIME/LATE`，不能安全声明 `reportId/publishTime` 的真实类型 |
| 统计与详情默认口径一致性 | REVIEW | 同次统计的三类计数均为 0，但默认日期的 `NOT_SUBMITTED` 详情返回 1 条 |

## 决策

- 不根据字段名猜测 `reportId/publishTime`，也不把只观察到的 `null` 扩大为提交成功行的稳定类型。
- 不用 `list-statistics` 的计数推断 `submit-detail` 的分页总数或默认日期语义；两个 endpoint 可能使用不同统计窗口。
- 命令继续留在 `agoal-out-of-surface` 精确 exclusion。取得明确 `--query-date` 下的非空三状态样本、分页边界和跨 endpoint 口径说明后，再进入 `dual_validate`。
- 这不是追求 exclusion=0 的阻塞项；Agoal 其他可独立验证的只读叶可以继续逐条准入。
