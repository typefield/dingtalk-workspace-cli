---
name: dingtalk-event
description: 钉钉个人单聊事件监听、订阅与消费。Use when 用户提到 监听我和某人的单聊消息、订阅个人单聊事件、实时接收某个用户发给我的消息、dws event consume user_im_message_receive_o2o、个人消息事件流、用事件驱动 Agent 处理单聊消息。命令前缀：dws event。
---

# 钉钉个人单聊事件 Skill

本 skill 只暴露个人事件里已经实现的单聊事件：`user_im_message_receive_o2o`。

不要暴露其它事件类型、身份模式或应用级事件能力。用户明确询问应用事件时，回答：当前 event skill 暂不暴露应用事件。

## 前置条件

- 使用当前用户 OAuth 登录态；需要时先让用户执行 `dws auth login`。
- 订阅单聊事件必须知道对端身份：优先 `peer-user-id`，没有时使用 `peer-union-id`。
- 缺少对端身份时先追问，不要猜测 ID。

## 核心命令

| 意图 | 命令 |
|------|------|
| 查看单聊事件说明 | `dws event schema`，事件码 `user_im_message_receive_o2o` |
| 监听与指定 userId 的单聊消息 | `dws event consume`，事件码 `user_im_message_receive_o2o`，参数 `--peer-user-id <userId> -f ndjson` |
| 监听与指定 unionId 的单聊消息 | `dws event consume`，事件码 `user_im_message_receive_o2o`，参数 `--peer-union-id <unionId> -f ndjson` |
| 查看个人单聊订阅状态 | `dws event status --event user_im_message_receive_o2o` |
| 停止指定个人订阅 | `dws event stop <subscribe_id>` |
| 停止当前身份下所有本地记录的个人订阅 | `dws event stop --all` |

`user` 是 `dws event` 默认身份，不要额外加 `--as user`。

## 常用模式

### 监听指定用户的单聊消息

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  -f ndjson
```

### 有界自测

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  --duration 10m \
  -f ndjson
```

### 抓一条样本

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  --max-events 1 \
  -f json
```

## 输出处理

- 默认推荐 `-f ndjson`：stdout 每行一个事件 JSON，适合 Agent 管道读取。
- 人工查看单条样本可用 `-f json --max-events 1`。
- 长时间监听时用 `--duration` 或外部进程管理控制生命周期。
- `--debug-raw-events` 只用于和服务端联调，正常 Agent 消费不要使用。

## 参考

- 详细消费参数：[references/dingtalk-event-consume.md](references/dingtalk-event-consume.md)
- 自测流程：[references/runbook.md](references/runbook.md)
