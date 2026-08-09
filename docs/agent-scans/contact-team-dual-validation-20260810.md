# contact +team dual validation Agent 审阅

扫描日期：2026-08-10

## Result: PASS

- `contact +team` 以 `legacy_only -> dual_validate` 单步进入影子校验。
- canonical `+list-dept-members` 与复合 `+team` 复用同一个成员投影，避免对 `deptUserList/userInfo` 和稳定 `userId` 做两套解释。
- 终结 MCP 调用只执行一次；legacy writer 继续输出原字节，ResultMapper 在内存验证 `count/members` 与 `meta.count`。
- 未知成员容器、非法行或缺稳定 `userId` 均 fail-closed，不会压成成功空/残缺列表。
