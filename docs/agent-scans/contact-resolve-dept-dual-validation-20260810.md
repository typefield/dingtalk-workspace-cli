# contact +resolve-dept dual validation Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；姓名和部门名仅在内存中作为下一步输入。本文件不保存查询词、
> 姓名、部门名、deptId 或原始 JSON，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 严格投影与 rollout 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Runtime Schema 固定消歧结果 | PASS | `properties=3, rc=0` |
| 真实当前部门消歧结构对拍 | PASS | `rc=0, legacy=yes, candidates=2, stable_ids=yes` |
| rollout 单步迁移 | PASS | `legacy_only -> dual_validate` |

## 结论

- dual 阶段的正常 JSON 仍是既有 `resolved/count/candidates` legacy payload，业务只调用一次；没有统一信封或版本字段泄漏。
- 未知容器、非法候选、缺失/分数/重复部门 ID 不再被静默丢弃或压成零命中。
- `hasMore:true`、缺失分页证据或终页 total 不对账会 fail-closed，不能据当前页判断唯一部门。
- 企业根部门搜索哨兵 `-1` 会归一为下游可直接使用的 canonical `deptId=1`。

## 边界

当前真实读取只覆盖一个产生两个候选的终页响应；它不证明零命中、唯一命中、根部门、
权限受限或服务端异常形状已经在真实环境出现。
