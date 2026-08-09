# contact +lookup 完整资料统一结果 Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；资料 JSON 只在内存解析。本文件不保存姓名、联系方式、工号、
> userId、部门/职位/标签内容或原始响应，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 完整投影与 fail-closed 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Runtime Schema 覆盖完整资料与敏感路径 | PASS | `properties=11, sensitive=5, rc=0` |
| 真实当前用户完整资料结构对拍 | PASS | `rc=0, fields=8, departments=1, positions=1, labels=0, stable_ids=yes` |
| rollout 单步迁移 | PASS | `dual_validate -> unified_active` |

## 结论

- `contact +lookup --format json` 返回统一 `ok/outcome/data`，不含协议选择参数、版本标记或原始 `orgEmployeeModel` 包装。
- 投影保留身份、组织、部门、职位、标签与联系方式，不以精简字段换取统一输出。
- 唯一用户、稳定 userId、三类数组及逐项稳定 ID 是成功前提；未知字段、类型漂移和别名冲突均 fail-closed。
- 上游当前会在没有组织 ID 时显式返回 `orgId:null`；由于公开 Schema 将 `organization.id` 定义为可选字段，投影会省略该 ID，但仍保留经过审阅的组织名称等字段。非空但非法的 ID 继续 fail-closed。

## 边界

当前用户成功只证明本次响应可完整投影，不证明其他用户、非空标签、权限受限或后端新增字段。
