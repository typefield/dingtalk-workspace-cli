# 工作流详细说明

## 阶段 1: 获取当前用户

1. 调用 `dws contact user get-self --format json`
2. 提取 `userId` 和 `name`，用于阶段 3 负责人匹配、阶段 4 执行人参数
3. 校验: 返回包含有效 userId
4. 失败: 认证失败 → 提示 `dws auth login` 后重试（最多两次）

## 阶段 2: 定位听记并提取待办

依赖: 无

### 决策点: 听记定位方式
- IF 用户说"最新 / 刚开完的会" → 直接 `dws minutes +action-items --format json`（内部自动取最新一条）
- IF 用户给了关键词（如"周会"）→
  1. `dws minutes +list-mine --query "周会" --limit 10 --format json`
  2. 从返回中取目标听记的 `taskUuid`；多条匹配时列出标题+时间请用户选择
  3. `dws minutes +detail --id <taskUuid> --artifacts todos --format json`
- IF 用户要"今天全部会议的待办" → `+list-mine --limit 20` 后按 createTime 本地筛今天的，逐条 `+detail --artifacts todos`

> **注意**：`+detail` 用 `--artifacts todos` 只拉待办产物，不要默认全量（会连逐字稿一起拉）。

校验: 至少有 1 条待办
失败: 无待办 → 告知「听记中未发现待办事项」，流程结束

## 阶段 3: 筛选我的待办

依赖: 阶段 1 的 userId/name + 阶段 2 的待办列表

1. 遍历待办，检查参与人是否匹配当前用户（userId 或姓名）
2. 无明确负责人的待办 → 询问用户是否认领
3. 用户要求"全部转换"时跳过筛选

### 决策点: 待办认领
- IF 无负责人且用户认领 → 加入待创建列表
- IF 无负责人且用户忽略 → 跳过
- IF 已是他人负责且用户未要求全转 → 跳过

校验: 筛选后至少 1 条
失败: 无我负责的待办 → 告知「未发现需要您负责的待办」

## 阶段 4: 创建钉钉待办

依赖: 阶段 3 的待创建列表

1. 展示待创建列表（标题 / 来源听记 / 截止时间）请用户确认
2. MUST: 获得确认后再执行；每条：
   ```bash
   dws todo task create --title "<待办内容>" --executors <userId> --format json
   ```
3. 有截止时间时加 `--due "<ISO-8601>"`；重要任务加 `--priority 40`
4. 单条失败不中断，记录失败原因继续下一条

## 阶段 5: 汇总报告

```
✅ 已创建 N 条待办：
- 「标题」（截止 MM-DD）
⏭ 跳过 M 条（他人负责/用户忽略）
⚠️ 失败 K 条：{原因}
```
