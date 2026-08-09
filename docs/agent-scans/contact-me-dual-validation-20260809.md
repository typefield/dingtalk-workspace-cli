# contact +me dual validation Agent 审阅

扫描日期：2026-08-09

> 本次只验证 `legacy_only -> dual_validate`。业务读取只执行一次，外部 stdout 保持既有 legacy 字节；统一结果只在内存中构造、校验，不保存 JSON fixture，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 证据 |
|---|---|---|
| 当前用户投影需要稳定 `userId` | PASS | 空结果、多记录和 display-only 均返回 `api/projection_unknown` |
| ResultSpec 可验证 | PASS | success/failure；`userId` required；`email/mobile` sensitive |
| 单次业务执行 | PASS | 测试 caller `calls=1` |
| legacy stdout 不变 | PASS | dual 阶段 exact-byte golden 通过，且没有 `ok/outcome` |
| rollout 单步迁移 | PASS | Agent rollout ledger：`legacy_only -> dual_validate` |

## 结论

- dual 阶段不向用户或 Agent 暴露协议选择参数；调用方式仍是 `dws contact +me --format json`。
- 当前阶段不宣称已经启用统一信封，只证明新投影/ResultSpec 可以在不改变旧 wire 的前提下通过严格校验。
- 下一步只有在 rollout ledger 以本阶段为基线、真实只读探针通过后，才允许单独迁移为 unified active。
