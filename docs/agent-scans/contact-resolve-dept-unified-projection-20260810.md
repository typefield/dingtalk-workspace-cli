# contact +resolve-dept unified active Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；姓名和部门名仅在内存中作为下一步输入。本文件不保存查询词、
> 姓名、部门名、deptId 或原始 JSON，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 严格投影与 rollout 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Runtime Schema 固定消歧结果 | PASS | `properties=3, rc=0` |
| 真实当前部门消歧结构对拍 | PASS | `rc=0, unified=yes, candidates=2, count_aligned=yes` |
| rollout 单步迁移 | PASS | `dual_validate -> unified_active` |

## 结论

- 普通 `contact +resolve-dept --format json` 现在返回统一 `ok/outcome/data/meta`；没有协议选择参数或版本字段。
- `data` 固定为 `resolved/count/candidates`，候选均带可操作的字符串 `deptId` 与清洁名称；`meta.count` 与 `meta.pagination.items` 对账，只有观察到 `hasMore:false` 且 total 相符时才声明 `endpoint_exhausted:true`。
- 未知容器、非法候选、缺失/分数/重复部门 ID 不再被静默丢弃或压成零命中。
- 企业根部门搜索哨兵 `-1` 会归一为下游可直接使用的 canonical `deptId=1`。

## 边界

当前真实读取只覆盖一个产生两个候选的终页响应；它不证明零命中、唯一命中、根部门、
权限受限或服务端异常形状已经在真实环境出现。
