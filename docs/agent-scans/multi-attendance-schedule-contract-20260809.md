# Multi Attendance 排班导入 Agent 语义探针

临时 child runner 验证脚本的确认门禁、只读预览、机器输出和终态诚实性；不证明真实租户排班已经生效。

| 检查 | 结果 | 证据 |
|---|---|---|
| Help 可发现确认、格式与预览语义 | PASS | rc=0 |
| 类型错误在任何远端调用前返回 typed validation | PASS | rc=1; ok |
| dry-run 仅做只读校验且不导入排班 | PASS | rc=0; ok; child_reads=3 |
| 仅精确读取已绑定班次名称，预览不展示裸 classId | PASS | class_get=True; global_search=False |
| 组绑定缺失 fail-closed，不回退企业全局班次目录 | PASS | rc=1; ok |
| 未确认只返回 preview 与 confirmation_required | PASS | rc=1; ok |
| 受确认写入只报告请求受理，不夸大为逐条终态 | PASS | rc=0; ok |
| 写请求异常保留 execution_state=unknown 且不建议盲重试 | PASS | rc=1; ok |

结论：**8/8 PASS**。

范围：验证 Help、输入门禁、dry-run、确认、请求受理和不确定写入；真实权限、人员归属、排班覆盖和最终记录须由隔离组织复验。
