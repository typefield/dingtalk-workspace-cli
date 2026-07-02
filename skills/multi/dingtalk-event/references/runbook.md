# 个人单聊事件自测流程

## 1. 确认登录

个人事件使用当前用户 OAuth 登录态。未登录或 token 失效时，先执行：

```bash
dws auth login
```

## 2. 查看单聊事件说明

```bash
dws event schema user_im_message_receive_o2o
```

确认事件规则是 `singleChat`，必填参数是 `peer-user-id` 或 `peer-union-id`。

## 3. 启动监听

优先用对端 `userId`：

```bash
dws event consume user_im_message_receive_o2o \
  --peer-user-id 507971 \
  --duration 10m \
  -f ndjson
```

如果只有 `unionId`：

```bash
dws event consume user_im_message_receive_o2o \
  --peer-union-id <unionId> \
  --duration 10m \
  -f ndjson
```

## 4. 触发事件

让对端用户给当前登录用户发送一条单聊消息。stdout 应输出一行事件 JSON。

如果没有输出：

1. 确认对端身份参数正确。
2. 用 `dws event status --event user_im_message_receive_o2o` 查看订阅和本地连接状态。
3. 联调服务端时临时加 `--debug --debug-raw-events`，观察当前 personal stream bus 是否收到服务端推送。

## 5. 停止监听

先查订阅 ID：

```bash
dws event status --event user_im_message_receive_o2o
```

停止指定订阅：

```bash
dws event stop <subscribe_id>
```

清理当前身份下本地记录的全部个人订阅：

```bash
dws event stop --all
```

## 6. 安装到 Agent

安装 multi skill 的 event 子 skill：

```bash
dws skill setup --mode multi -s event --target codex --source <repo> --yes
```

安装 mono skill：

```bash
dws skill setup --mode mono --target codex --source <repo> --yes
```
