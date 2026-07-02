# `dws event` — 个人单聊事件

通过个人 Stream 长连接监听当前用户与指定用户的单聊消息，NDJSON 输出到 stdout，用于驱动事件触发的 Agent。

当前只暴露 `user_im_message_receive_o2o`。

## 当用户说……

| 用户说 | 命令 |
|--------|------|
| "监听我和 507971 的单聊消息" | `dws event consume`，事件码 `user_im_message_receive_o2o`，参数 `--peer-user-id 507971 -f ndjson` |
| "订阅某个 unionId 的单聊事件" | `dws event consume`，事件码 `user_im_message_receive_o2o`，参数 `--peer-union-id <unionId> -f ndjson` |
| "查看个人单聊事件 schema" | `dws event schema`，事件码 `user_im_message_receive_o2o` |
| "看个人单聊订阅状态" | `dws event status --event user_im_message_receive_o2o` |
| "停止这个个人事件订阅" | `dws event stop <subscribe_id>` |

## 核心规则

- `user` 是默认身份，不要加 `--as user`。
- 必须提供对端身份：优先 `--peer-user-id`，没有时使用 `--peer-union-id`。
- 缺少对端身份时先追问，不要猜测。
- 不要主动运行 `dws event list` 作为能力菜单；本参考只承认 `user_im_message_receive_o2o`。
- 当前 event 参考不暴露应用事件。用户明确询问应用事件时，回答“当前 event skill 暂不暴露应用事件”。

## 常用命令

```bash
dws event schema user_im_message_receive_o2o
```

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  -f ndjson
```

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  --duration 10m \
  -f ndjson
```

```bash
dws event status --event user_im_message_receive_o2o
dws event stop <subscribe_id>
```

## 输出格式

- 推荐 `-f ndjson`：一行一个事件 JSON，适合 Agent 管道读取。
- 人工取样可用 `-f json --max-events 1`。
- `--debug-raw-events` 仅用于服务端联调，正常消费不要使用。

## 完整文档

详细见 multi 模式：`skills/multi/dingtalk-event/SKILL.md`
自测流程：`skills/multi/dingtalk-event/references/runbook.md`
