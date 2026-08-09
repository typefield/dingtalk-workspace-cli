# IM 分页统一返回 Agent 审阅

扫描日期：2026-08-09  
范围：`chat` 的首批六条分页 terminal command；不调用真实 IM，不保存运行 JSON fixture。

## 审阅结论

首批六条命令都已处于 `unified_active`，而不是 RFC-0004 中先前写的
`dual_validate`。Agent 只使用 `--format json`；没有公开的协议版本选择参数。

| canonical command | 当前声明位置 | 统一分页语义的本地证据 |
|---|---|---|
| `chat +chat-search` | `internal/shortcut/chat/chat_group.go` | 续页、未知证据、后续页失败、窗口探测和游标矛盾回归 |
| `chat +flag-list` | `internal/shortcut/chat/lark_alignment.go` | 续页、未知证据、后续页失败和游标停滞回归 |
| `chat +conversation-list` | `internal/shortcut/chat/chat_conversation.go` | 终态零游标、预算续页、未知容器和后续页失败回归 |
| `chat +thread-replies` | `internal/shortcut/smart/thread_replies.go` | 耗尽/续页/未知/partial、资源下载失败和 legacy byte 对拍 |
| `chat +chat-messages` | `internal/shortcut/smart/chat_messages.go` | 耗尽/续页/未知/partial、资源下载/本地导出失败和 legacy byte 对拍 |
| `chat +search-msg` | `internal/shortcut/smart/search_msg.go` | 默认多维检索；耗尽/续页/未知、后续页/富化/资源失败、严格消息投影 |

本次 Agent 在当前工作树执行的定向回归均通过：

- `go test -count=1 ./internal/shortcut/chat`：前三条命令的 active 声明，终态/续页/未知、游标矛盾、后续页失败与 `partial_failure` 覆盖通过；
- `go test -count=1 ./internal/shortcut/smart`：`+thread-replies`、`+chat-messages` 与 `+search-msg` 的 active 统一结果、资源/导出失败的 partial 表达，以及历史 dual legacy byte 对拍通过。

`+search-msg` 的扫描还确认：它此前把 `contractVersion` 和自定义分页/部分失败字段混进 payload；当前 active data 不再暴露这些字段，统一 `outcome` 和 `meta.pagination` 是唯一终态表达，但保留 `indexCoverageKnown:false` 作为搜索索引覆盖未知的业务事实。

这些是 Agent 的源码与受控执行审阅，不是 CI/policy gate，也不证明真实 IM 索引完整、真实分页字段稳定或资源下载字节完整。真实账号复验应至少覆盖：正常多页、空结果、服务端 `hasMore` 与游标冲突、第二页网络失败，以及资源下载/本地导出失败后的实际文件状态。
