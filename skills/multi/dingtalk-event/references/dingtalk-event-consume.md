# `dws event consume` 个人单聊参考

本参考只覆盖个人单聊事件 `user_im_message_receive_o2o`。

## Synopsis

```bash
dws event consume user_im_message_receive_o2o [flags]
```

## 必填参数

必须二选一：

| 参数 | 说明 |
|------|------|
| `--peer-user-id <userId>` | 对端用户的 userId，优先使用 |
| `--peer-union-id <unionId>` | 对端用户的 unionId |

缺少对端身份时，先向用户追问。

## 常用参数

| 参数 | 说明 |
|------|------|
| `-f, --format ndjson` | 推荐输出，一行一个事件 JSON |
| `--max-events <n>` | 收到 N 条后退出 |
| `--duration <duration>` | 到时退出，如 `30s`、`10m` |
| `--output-dir <dir>` | 每个事件写入一个文件 |
| `--route '<regex>=dir:<path>'` | 按事件类型路由到目录 |
| `--subscribe-id <id>` | 复用已有个人订阅 |
| `--personal-event-base-url <url>` | 联调环境覆盖控制面 base URL |
| `--stream-ticket-url <url>` | 联调环境覆盖取票 URL |
| `--stream-source-id <id>` | 联调环境覆盖 sourceId |
| `--debug-raw-events` | 联调用：输出当前 personal stream bus 收到的全部可解析事件 |

不要使用未在本参考中列出的事件选择参数；本 skill 只消费 `user_im_message_receive_o2o`。

## 输出

`-f ndjson` 每行是一个事件对象，常见字段：

| 字段 | 说明 |
|------|------|
| `event_type` | 应为 `user_im_message_receive_o2o` |
| `subscribe_id` | 个人订阅 ID |
| `source_id` | 当前 sourceId |
| `data` | 服务端业务 payload 原文 |
| `headers` | Stream 帧 headers |
| `received_at_unix_ms` | 本地接收时间 |

业务消息内容通常在 `data` 内部 payload 中，读取前先查看实际样本。

## 示例

### 持续监听

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  -f ndjson
```

### 监听 10 分钟

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  --duration 10m \
  -f ndjson
```

### 获取一条 JSON 样本

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  --max-events 1 \
  -f json
```

### 联调预发环境

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  --personal-event-base-url https://pre-mcp.dingtalk.com/dws \
  --stream-ticket-mode normal \
  --stream-source-id pre_open_source \
  --stream-ticket-url https://pre-mcp.dingtalk.com/stream/connections/ticket \
  -f ndjson
```

## 停止

```bash
dws event status --event user_im_message_receive_o2o
dws event stop <subscribe_id>
```

如果只想清理当前身份下本地记录的所有个人订阅：

```bash
dws event stop --all
```
