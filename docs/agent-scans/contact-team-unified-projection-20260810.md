# contact +team 统一结果 Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；用户和成员数据只在内存解析。本文件不保存姓名、userId、部门信息或原始 JSON，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 共享成员投影与统一结果回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| 真实当前部门成员结构对拍 | PASS | `rc=0, count=19, stable_user_ids=yes, meta_aligned=yes` |

## 结论

- `contact +team --format json` 只返回 `ok/outcome/data/meta`，没有协议选择参数或版本标记。
- canonical 部门成员命令与复合入口共用唯一投影；`data.count == meta.count == members.length`。
- 每条成员必须有稳定 `userId`，未知或残缺条目使整个结果 fail-closed，不伪装为空或完整。
- 当前部门读取不证明下级部门、全组织覆盖、其他账号或权限受限路径。
