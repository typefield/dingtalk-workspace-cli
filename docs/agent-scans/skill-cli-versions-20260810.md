# Skill version / cli_version Agent 扫描

此文件由 Agent 扫描生成；不作为运行时 JSON fixture，也不接入 CI 门禁。

- Skill 数量：14
- Skill 版本声明集合：1.0.0, 1.0.1, 1.0.5, 1.0.6
- CLI 版本声明集合：>=1.0.15
- Skill 版本格式：PASS
- CLI 版本覆盖：PASS

| Skill | version | cli_version |
|---|---|---|
| `skills/mono/SKILL.md` | `1.0.5` | `>=1.0.15` |
| `skills/multi/dingtalk-aisearch/SKILL.md` | `1.0.0` | `>=1.0.15` |
| `skills/multi/dingtalk-aitable/SKILL.md` | `1.0.1` | `>=1.0.15` |
| `skills/multi/dingtalk-calendar/SKILL.md` | `1.0.0` | `>=1.0.15` |
| `skills/multi/dingtalk-chat/SKILL.md` | `1.0.0` | `>=1.0.15` |
| `skills/multi/dingtalk-contact/SKILL.md` | `1.0.0` | `>=1.0.15` |
| `skills/multi/dingtalk-doc/SKILL.md` | `1.0.1` | `>=1.0.15` |
| `skills/multi/dingtalk-drive/SKILL.md` | `1.0.1` | `>=1.0.15` |
| `skills/multi/dingtalk-mail/SKILL.md` | `1.0.0` | `>=1.0.15` |
| `skills/multi/dingtalk-minutes/SKILL.md` | `1.0.0` | `>=1.0.15` |
| `skills/multi/dingtalk-misc/SKILL.md` | `1.0.6` | `>=1.0.15` |
| `skills/multi/dingtalk-shared/SKILL.md` | `1.0.1` | `>=1.0.15` |
| `skills/multi/dingtalk-todo/SKILL.md` | `1.0.0` | `>=1.0.15` |
| `skills/multi/dingtalk-wiki/SKILL.md` | `1.0.0` | `>=1.0.15` |

结论：每个 Skill 均有独立语义版本，并声明最低 CLI 兼容版本；两者语义不能互相替代。
