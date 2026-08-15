# attendance 局部意图消歧

本文件从单 Skill `intent-guide.md` 拆分而来，仅保留与本产品相关的跨产品消歧规则。

| 用户说... | 真实意图 | 应该用 | 不要用 | 理由 |
|---|---|---|---|---|
| "查/提交 请假/加班/外出/出差/补卡 审批单" | 考勤业务审批单 | `attendance approve`（查询走 `attendance approve list`；提交走 `attendance approve templates --type leave\|overtime\|repair-check\|travel\|out`） | `oa approval list-pending` / `oa approval records` | 请假/加班/外出/出差/补卡这 5 类属于考勤业务审批单，按业务类型查询；`oa approval` 是通用 OA 审批中心，覆盖范围不同 |
| "我的待审批 / 待办审批 / 已审批记录" | 通用 OA 审批任务 | `oa approval list-pending` | `attendance approve` | 不限于考勤业务、面向"我上下游的审批任务"的表述走通用审批中心 |
| "报销 / 采购 / 用印 / 合同 等非考勤类审批" | 通用 OA 审批单 | `oa approval detail/approve/reject/revoke` | `attendance approve` | 非考勤业务审批单走 `oa approval` |

## 判断关键

- 审批主题明确是【请假 / 加班 / 外出 / 出差 / 补卡】这 5 类 → `attendance approve`。`trip` 在查询接口 bizType=2 同时覆盖出差与外出（travel / business_trip / 出差 / 外出 亦映射到 2）；外出=travel/TRAVEL，出差=out/trip/OUT。
- 不限于考勤业务、面向"我上下游的审批任务"表述 → `oa approval`。

## 提交审批单的边界

- 提交考勤审批单走 `attendance approve templates --type leave|overtime|repair-check|travel|out`，命令会返回审批表单的 submitUrl 跳转链接，由用户点击链接跳转到钉钉客户端的提交页面完成填写与提交。**展示链接时必须用 Markdown 可点击格式 `[表单名称](submitUrl)`，不要裸露 URL**。
- 提交诉求的辅助查询：可用假期余额走 `attendance vacation balance`、历史已提交记录走 `attendance approve list`。
- 任何场景下都**不要误用 `oa approval` 代替**——该命令组只能查/审/撤已存在的审批单，考勤业务审批单走考勤自己的逻辑便于区分。
