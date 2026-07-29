# 工作流详细说明

## 阶段 1: 获取今日日程

1. 调用 `dws calendar +agenda --format json`（默认今天 00:00~23:59，无需时间参数）
2. 用户指定其他日期时，AI 先换算 ISO-8601：
   ```bash
   dws calendar +agenda --start "2026-07-30T00:00:00+08:00" --end "2026-07-30T23:59:59+08:00" --format json
   ```
3. 校验: 返回 JSON 可解析；提取每场日程的 summary / start / end / 地点
4. 失败: 查询报错 → 跳过本模块，日报中不展示日程段落

### 决策点: 时间范围选择
- IF 用户指定了日期 → 换算 ISO-8601 传 `--start/--end`
- IF 用户说"这周" → start=本周一 00:00，end=本周日 23:59
- ELSE → 默认今天（不传参数）

> **注意**：`--start/--end` 不支持 `"tomorrow"` 等自然语言，必须由 AI 换算。

## 阶段 2: 获取未完成待办

依赖: 无（与阶段 1 并行安全，只读）

1. 调用 `dws todo +get-my-tasks --status false --format json`
2. 数据量大时按需过滤：
   - 高优先级优先：`--priority 40,30`
   - 截止今天之前：`--plan-finish-end <今天 23:59:59 的 Unix 毫秒>`
3. 校验: 返回含 todos 列表
4. 失败: 跳过本模块

> **注意**：不带 `--status` 会同时返回已完成和未完成，摘要场景**必须**带 `--status false`。
> `--plan-finish-start/--plan-finish-end` 是 Unix 毫秒时间戳，不是 ISO-8601。
> `--role-types` 默认 executor；需要包含我创建的加 `--role-types creator,executor`。

## 阶段 3: 获取待处理审批

依赖: 无（并行安全，只读）

1. 调用 `dws oa +list-pending --format json`
2. 提取流程标题 / 发起人 / 发起时间
3. 失败: 跳过本模块

## 阶段 4: 获取最新会后待办（可选）

依赖: 无（并行安全，只读）

1. 仅当用户提到「会议 / 开会 / 听记」或要求完整日报时执行
2. 调用 `dws minutes +action-items --format json`
3. 返回「暂无妙记」时跳过本模块，不视为失败

## 阶段 5: AI 汇总

依赖: 阶段 1-4 的全部结果

1. 时间戳统一转为本地时区 `HH:MM`
2. 日程重叠检测: 前一场 end > 后一场 start 即标 ⚠️
3. 待办排序: 先按优先级降序（40=P1 最高），再按截止时间升序；无截止时间排最后
4. 待办超过 20 项只展示前 20，其余折叠为「其他 N 项」
5. 任一模块为空则整段跳过，不输出空段落
6. 按 SKILL.md 的「输出模板」格式化

### 失败汇总规则

- 全部模块失败 → 提示检查登录状态（`dws auth status`），不输出空日报
- 部分失败 → 正常输出成功模块，结尾加一行「⚠️ {模块名} 查询失败已跳过」
