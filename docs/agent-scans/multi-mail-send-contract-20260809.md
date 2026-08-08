# Multi Mail 带抄送发送 Agent 语义探针

临时 child runner 仅验证编排与结果表达；本报告不保存 JSON fixture，也不向真实邮箱发送邮件。

| 检查 | 结果 | 证据 |
|---|---|---|
| 脚本 Help 可发现 format/dry-run/yes | PASS | rc=0 |
| dry-run 零 child 调用且返回单信封 | PASS | rc=0; ok; sentinel=False |
| 未确认发送 fail-closed | PASS | rc=1; ok; sentinel=False |
| 输入校验错误不泄漏 traceback | PASS | rc=1; ok |
| 旧业务 success:false 不误报已发送 | PASS | rc=1; ok |
| 发送后仅在 verify=success 时报告已验证 | PASS | rc=0; ok |
| 投递中不伪装终态成功 | PASS | rc=1; ok |
| 无逐收件人明细的部分投递不伪造 partial 三通道 | PASS | rc=1; ok |

结论：**8/8 PASS**。

范围：验证 Help、确认门禁、零写预览、旧业务失败、发送后 readback 与未终态表达；真实投递仍须隔离账号复验。
