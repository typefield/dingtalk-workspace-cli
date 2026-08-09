# contact +org 统一结果 Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；用户与部门数据只在内存解析。本文件不保存姓名、userId、部门名称、deptId 或原始 JSON，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| ResultMapper/投影/legacy 兼容回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| 真实当前用户部门结构对拍 | PASS | `rc=0, fields=3, stable_dept_id=yes, stderr=empty` |

## 结论

- `contact +org --format json` 直接返回单一 `ok/outcome/data`，没有协议选择参数或版本标记。
- `data` 只含 `deptId/deptName/memberCount`，稳定 `deptId` 是成功必需条件；未知形状 fail-closed。
- ResultMapper 让 dual 阶段复用 legacy writer、active 阶段使用审阅投影，业务请求始终 exactly-once。
- 本次当前用户读取不证明整个组织目录、其他身份或权限受限路径。
