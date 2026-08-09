# Unified output rollout Agent ledger

扫描时间：2026-08-09T15:30:00+08:00

> 本报告由 Agent 在隔离配置目录中装配真实 Cobra tree 后生成。它只记录 Markdown，不暴露内部 rollout 到 Help/Schema/CLI，不保存 JSON catalog，也不是 CI / policy gate。

## 当前 inventory

| rollout state | runnable command nodes |
|---|---:|
| `legacy_only` | 1350 |
| `dual_validate` | 20 |
| `unified_active` | 113 |
| `unified_stable` | 0 |
| `unified_only` | 0 |

可执行命令节点总数：**1483**。其中可能包含兼具子命令与直接执行语义的 Cobra 节点；它不等同于公开 Schema 工具计数。

## Transition review

基线：`rollout-ledger-before-sheet-list-active.md`（1483 条 runnable command node）。

### 状态迁移

- PASS: `dws sheet +list-sheets`: `dual_validate` → `unified_active`

### 新增可执行命令节点

无新增可执行命令节点。

### 移除可执行命令节点

无移除可执行命令节点。

## Live command declaration

| cli path | rollout state | active contract | hidden |
|---|---|---|---|
| `dws agoal` | `legacy_only` | `legacy` | no |
| `dws agoal contract` | `legacy_only` | `legacy` | no |
| `dws agoal contract detail` | `legacy_only` | `legacy` | no |
| `dws agoal contract fields` | `legacy_only` | `legacy` | no |
| `dws agoal contract list` | `legacy_only` | `legacy` | no |
| `dws agoal contract update` | `legacy_only` | `legacy` | no |
| `dws agoal obj-template` | `legacy_only` | `legacy` | no |
| `dws agoal obj-template create-or-update` | `legacy_only` | `legacy` | no |
| `dws agoal obj-template list` | `legacy_only` | `legacy` | no |
| `dws agoal report` | `legacy_only` | `legacy` | no |
| `dws agoal report list-statistics` | `legacy_only` | `legacy` | no |
| `dws agoal report submit-detail` | `legacy_only` | `legacy` | no |
| `dws agoal scorecard` | `legacy_only` | `legacy` | no |
| `dws agoal scorecard detail` | `legacy_only` | `legacy` | no |
| `dws agoal scorecard entity-detail` | `legacy_only` | `legacy` | no |
| `dws agoal scorecard update` | `legacy_only` | `legacy` | no |
| `dws agoal strategy` | `legacy_only` | `legacy` | no |
| `dws agoal strategy detail` | `legacy_only` | `legacy` | no |
| `dws agoal strategy list` | `legacy_only` | `legacy` | no |
| `dws agoal strategy update` | `legacy_only` | `legacy` | no |
| `dws agoal user` | `legacy_only` | `legacy` | no |
| `dws agoal user objectives` | `legacy_only` | `legacy` | no |
| `dws agoal user rules` | `legacy_only` | `legacy` | no |
| `dws aisearch` | `legacy_only` | `legacy` | no |
| `dws aisearch behavior` | `legacy_only` | `legacy` | no |
| `dws aisearch enterprise` | `legacy_only` | `legacy` | no |
| `dws aisearch person` | `legacy_only` | `legacy` | no |
| `dws aitable` | `legacy_only` | `legacy` | no |
| `dws aitable +advperm-disable` | `legacy_only` | `legacy` | no |
| `dws aitable +advperm-enable` | `legacy_only` | `legacy` | no |
| `dws aitable +attachment-put` | `legacy_only` | `legacy` | no |
| `dws aitable +attachment-remove` | `legacy_only` | `legacy` | no |
| `dws aitable +attachment-upload` | `legacy_only` | `legacy` | no |
| `dws aitable +base-bootstrap` | `legacy_only` | `legacy` | no |
| `dws aitable +base-copy` | `legacy_only` | `legacy` | no |
| `dws aitable +base-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +base-get` | `legacy_only` | `legacy` | no |
| `dws aitable +base-get-primary-doc-id` | `legacy_only` | `legacy` | no |
| `dws aitable +base-list` | `legacy_only` | `legacy` | no |
| `dws aitable +base-schema-snapshot` | `legacy_only` | `legacy` | no |
| `dws aitable +base-search` | `legacy_only` | `legacy` | no |
| `dws aitable +base-update` | `legacy_only` | `legacy` | no |
| `dws aitable +chart-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +chart-get` | `legacy_only` | `legacy` | no |
| `dws aitable +chart-share-get` | `legacy_only` | `legacy` | no |
| `dws aitable +chart-share-update` | `legacy_only` | `legacy` | no |
| `dws aitable +chart-update` | `legacy_only` | `legacy` | no |
| `dws aitable +chart-widgets-example` | `legacy_only` | `legacy` | no |
| `dws aitable +dashboard-arrange` | `legacy_only` | `legacy` | no |
| `dws aitable +dashboard-config-example` | `legacy_only` | `legacy` | no |
| `dws aitable +dashboard-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +dashboard-get` | `legacy_only` | `legacy` | no |
| `dws aitable +dashboard-share-get` | `legacy_only` | `legacy` | no |
| `dws aitable +dashboard-share-update` | `legacy_only` | `legacy` | no |
| `dws aitable +dashboard-update` | `legacy_only` | `legacy` | no |
| `dws aitable +export-data` | `legacy_only` | `legacy` | no |
| `dws aitable +field-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +field-get` | `legacy_only` | `legacy` | no |
| `dws aitable +field-update` | `legacy_only` | `legacy` | no |
| `dws aitable +find-record` | `legacy_only` | `legacy` | no |
| `dws aitable +form-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +form-field-hide` | `legacy_only` | `legacy` | no |
| `dws aitable +form-field-list` | `legacy_only` | `legacy` | no |
| `dws aitable +form-field-update` | `legacy_only` | `legacy` | no |
| `dws aitable +form-list` | `legacy_only` | `legacy` | no |
| `dws aitable +form-share-get` | `legacy_only` | `legacy` | no |
| `dws aitable +form-share-update` | `legacy_only` | `legacy` | no |
| `dws aitable +form-update` | `legacy_only` | `legacy` | no |
| `dws aitable +import-data` | `legacy_only` | `legacy` | no |
| `dws aitable +import-upload` | `legacy_only` | `legacy` | no |
| `dws aitable +list-tables` | `legacy_only` | `legacy` | no |
| `dws aitable +record-bulk-patch` | `legacy_only` | `legacy` | no |
| `dws aitable +record-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +record-history-list` | `legacy_only` | `legacy` | no |
| `dws aitable +record-primary-doc-create` | `legacy_only` | `legacy` | no |
| `dws aitable +record-primary-doc-get` | `legacy_only` | `legacy` | no |
| `dws aitable +record-query` | `legacy_only` | `legacy` | no |
| `dws aitable +record-query-empty` | `legacy_only` | `legacy` | no |
| `dws aitable +record-share-links` | `legacy_only` | `legacy` | no |
| `dws aitable +record-share-url` | `legacy_only` | `legacy` | no |
| `dws aitable +record-update` | `legacy_only` | `legacy` | no |
| `dws aitable +record-upsert` | `legacy_only` | `legacy` | no |
| `dws aitable +record-upsert-by-key` | `legacy_only` | `legacy` | no |
| `dws aitable +resolve-base` | `legacy_only` | `legacy` | no |
| `dws aitable +resolve-table` | `legacy_only` | `legacy` | no |
| `dws aitable +role-create` | `legacy_only` | `legacy` | no |
| `dws aitable +role-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +role-get` | `legacy_only` | `legacy` | no |
| `dws aitable +role-list` | `legacy_only` | `legacy` | no |
| `dws aitable +role-update` | `legacy_only` | `legacy` | no |
| `dws aitable +section-create` | `legacy_only` | `legacy` | no |
| `dws aitable +section-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +section-list-empty` | `legacy_only` | `legacy` | no |
| `dws aitable +section-list-nodes` | `legacy_only` | `legacy` | no |
| `dws aitable +section-move-node` | `legacy_only` | `legacy` | no |
| `dws aitable +section-rename` | `legacy_only` | `legacy` | no |
| `dws aitable +section-reorder` | `legacy_only` | `legacy` | no |
| `dws aitable +table-copy` | `legacy_only` | `legacy` | no |
| `dws aitable +table-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +table-get` | `legacy_only` | `legacy` | no |
| `dws aitable +table-update` | `legacy_only` | `legacy` | no |
| `dws aitable +template-search` | `legacy_only` | `legacy` | no |
| `dws aitable +url-resolve` | `legacy_only` | `legacy` | no |
| `dws aitable +view-delete` | `legacy_only` | `legacy` | no |
| `dws aitable +view-duplicate` | `legacy_only` | `legacy` | no |
| `dws aitable +view-get` | `legacy_only` | `legacy` | no |
| `dws aitable +view-get-frozen-cols` | `legacy_only` | `legacy` | no |
| `dws aitable +view-get-lock` | `legacy_only` | `legacy` | no |
| `dws aitable +view-get-row-height` | `legacy_only` | `legacy` | no |
| `dws aitable +view-lock` | `legacy_only` | `legacy` | no |
| `dws aitable +view-preset-apply` | `legacy_only` | `legacy` | no |
| `dws aitable +view-set-fill-color-rule` | `legacy_only` | `legacy` | no |
| `dws aitable +view-set-frozen-cols` | `legacy_only` | `legacy` | no |
| `dws aitable +view-set-row-height` | `legacy_only` | `legacy` | no |
| `dws aitable +view-update` | `legacy_only` | `legacy` | no |
| `dws aitable +workflow-deploy` | `legacy_only` | `legacy` | no |
| `dws aitable +workflow-disable` | `legacy_only` | `legacy` | no |
| `dws aitable +workflow-enable` | `legacy_only` | `legacy` | no |
| `dws aitable +workflow-get` | `legacy_only` | `legacy` | no |
| `dws aitable +workflow-list` | `legacy_only` | `legacy` | no |
| `dws aitable advperm` | `legacy_only` | `legacy` | no |
| `dws aitable advperm disable` | `legacy_only` | `legacy` | no |
| `dws aitable advperm enable` | `legacy_only` | `legacy` | no |
| `dws aitable advperm role-create` | `legacy_only` | `legacy` | no |
| `dws aitable advperm role-delete` | `legacy_only` | `legacy` | no |
| `dws aitable advperm role-get` | `legacy_only` | `legacy` | no |
| `dws aitable advperm role-list` | `legacy_only` | `legacy` | no |
| `dws aitable advperm role-update` | `legacy_only` | `legacy` | no |
| `dws aitable attachment` | `legacy_only` | `legacy` | no |
| `dws aitable attachment upload` | `legacy_only` | `legacy` | no |
| `dws aitable base` | `legacy_only` | `legacy` | no |
| `dws aitable base copy` | `legacy_only` | `legacy` | no |
| `dws aitable base create` | `legacy_only` | `legacy` | no |
| `dws aitable base delete` | `legacy_only` | `legacy` | no |
| `dws aitable base get` | `legacy_only` | `legacy` | no |
| `dws aitable base get-primary-doc-id` | `legacy_only` | `legacy` | no |
| `dws aitable base list` | `legacy_only` | `legacy` | no |
| `dws aitable base search` | `legacy_only` | `legacy` | no |
| `dws aitable base update` | `legacy_only` | `legacy` | no |
| `dws aitable chart` | `legacy_only` | `legacy` | no |
| `dws aitable chart create` | `legacy_only` | `legacy` | no |
| `dws aitable chart delete` | `legacy_only` | `legacy` | no |
| `dws aitable chart get` | `legacy_only` | `legacy` | no |
| `dws aitable chart share` | `legacy_only` | `legacy` | no |
| `dws aitable chart share get` | `legacy_only` | `legacy` | no |
| `dws aitable chart share update` | `legacy_only` | `legacy` | no |
| `dws aitable chart update` | `legacy_only` | `legacy` | no |
| `dws aitable chart widgets-example` | `legacy_only` | `legacy` | no |
| `dws aitable create` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard arrange` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard config-example` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard create` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard delete` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard get` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard share` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard share get` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard share update` | `legacy_only` | `legacy` | no |
| `dws aitable dashboard update` | `legacy_only` | `legacy` | no |
| `dws aitable doc` | `legacy_only` | `legacy` | yes |
| `dws aitable export` | `legacy_only` | `legacy` | no |
| `dws aitable export data` | `legacy_only` | `legacy` | no |
| `dws aitable field` | `legacy_only` | `legacy` | no |
| `dws aitable field create` | `legacy_only` | `legacy` | no |
| `dws aitable field delete` | `legacy_only` | `legacy` | no |
| `dws aitable field get` | `legacy_only` | `legacy` | no |
| `dws aitable field list` | `legacy_only` | `legacy` | no |
| `dws aitable field search-options` | `legacy_only` | `legacy` | no |
| `dws aitable field update` | `legacy_only` | `legacy` | no |
| `dws aitable form` | `legacy_only` | `legacy` | no |
| `dws aitable form create` | `legacy_only` | `legacy` | no |
| `dws aitable form delete` | `legacy_only` | `legacy` | no |
| `dws aitable form field` | `legacy_only` | `legacy` | no |
| `dws aitable form field hide` | `legacy_only` | `legacy` | no |
| `dws aitable form field list` | `legacy_only` | `legacy` | no |
| `dws aitable form field update` | `legacy_only` | `legacy` | no |
| `dws aitable form get` | `legacy_only` | `legacy` | no |
| `dws aitable form list` | `legacy_only` | `legacy` | no |
| `dws aitable form questions` | `legacy_only` | `legacy` | no |
| `dws aitable form questions create` | `legacy_only` | `legacy` | no |
| `dws aitable form questions delete` | `legacy_only` | `legacy` | no |
| `dws aitable form share` | `legacy_only` | `legacy` | no |
| `dws aitable form share get` | `legacy_only` | `legacy` | no |
| `dws aitable form share update` | `legacy_only` | `legacy` | no |
| `dws aitable form update` | `legacy_only` | `legacy` | no |
| `dws aitable import` | `legacy_only` | `legacy` | no |
| `dws aitable import data` | `legacy_only` | `legacy` | no |
| `dws aitable import upload` | `legacy_only` | `legacy` | no |
| `dws aitable info` | `legacy_only` | `legacy` | no |
| `dws aitable list` | `legacy_only` | `legacy` | no |
| `dws aitable record` | `legacy_only` | `legacy` | no |
| `dws aitable record batch-update` | `legacy_only` | `legacy` | no |
| `dws aitable record create` | `legacy_only` | `legacy` | no |
| `dws aitable record delete` | `legacy_only` | `legacy` | no |
| `dws aitable record get` | `legacy_only` | `legacy` | no |
| `dws aitable record history-list` | `legacy_only` | `legacy` | no |
| `dws aitable record list` | `legacy_only` | `legacy` | no |
| `dws aitable record primary-doc-create` | `legacy_only` | `legacy` | no |
| `dws aitable record primary-doc-get` | `legacy_only` | `legacy` | no |
| `dws aitable record query` | `legacy_only` | `legacy` | no |
| `dws aitable record query-empty` | `legacy_only` | `legacy` | no |
| `dws aitable record share-url` | `legacy_only` | `legacy` | no |
| `dws aitable record update` | `legacy_only` | `legacy` | no |
| `dws aitable record upsert` | `legacy_only` | `legacy` | no |
| `dws aitable search` | `legacy_only` | `legacy` | no |
| `dws aitable section` | `legacy_only` | `legacy` | no |
| `dws aitable section create` | `legacy_only` | `legacy` | no |
| `dws aitable section delete` | `legacy_only` | `legacy` | no |
| `dws aitable section list-empty` | `legacy_only` | `legacy` | no |
| `dws aitable section list-nodes` | `legacy_only` | `legacy` | no |
| `dws aitable section move-node` | `legacy_only` | `legacy` | no |
| `dws aitable section rename` | `legacy_only` | `legacy` | no |
| `dws aitable section reorder` | `legacy_only` | `legacy` | no |
| `dws aitable table` | `legacy_only` | `legacy` | no |
| `dws aitable table create` | `legacy_only` | `legacy` | no |
| `dws aitable table delete` | `legacy_only` | `legacy` | no |
| `dws aitable table get` | `legacy_only` | `legacy` | no |
| `dws aitable table list` | `legacy_only` | `legacy` | no |
| `dws aitable table update` | `legacy_only` | `legacy` | no |
| `dws aitable template` | `legacy_only` | `legacy` | no |
| `dws aitable template search` | `legacy_only` | `legacy` | no |
| `dws aitable view` | `legacy_only` | `legacy` | no |
| `dws aitable view create` | `legacy_only` | `legacy` | no |
| `dws aitable view delete` | `legacy_only` | `legacy` | no |
| `dws aitable view duplicate` | `legacy_only` | `legacy` | no |
| `dws aitable view get` | `legacy_only` | `legacy` | no |
| `dws aitable view get aggregate` | `legacy_only` | `legacy` | no |
| `dws aitable view get card` | `legacy_only` | `legacy` | no |
| `dws aitable view get field-widths` | `legacy_only` | `legacy` | no |
| `dws aitable view get fill-color-rule` | `legacy_only` | `legacy` | no |
| `dws aitable view get filter` | `legacy_only` | `legacy` | no |
| `dws aitable view get frozen-cols` | `legacy_only` | `legacy` | no |
| `dws aitable view get group` | `legacy_only` | `legacy` | no |
| `dws aitable view get lock` | `legacy_only` | `legacy` | no |
| `dws aitable view get row-height` | `legacy_only` | `legacy` | no |
| `dws aitable view get sort` | `legacy_only` | `legacy` | no |
| `dws aitable view get timebar` | `legacy_only` | `legacy` | no |
| `dws aitable view get visible-fields` | `legacy_only` | `legacy` | no |
| `dws aitable view list` | `legacy_only` | `legacy` | no |
| `dws aitable view lock` | `legacy_only` | `legacy` | no |
| `dws aitable view update` | `legacy_only` | `legacy` | no |
| `dws aitable view update aggregate` | `legacy_only` | `legacy` | no |
| `dws aitable view update card` | `legacy_only` | `legacy` | no |
| `dws aitable view update field-widths` | `legacy_only` | `legacy` | no |
| `dws aitable view update fill-color-rule` | `legacy_only` | `legacy` | no |
| `dws aitable view update filter` | `legacy_only` | `legacy` | no |
| `dws aitable view update frozen-cols` | `legacy_only` | `legacy` | no |
| `dws aitable view update group` | `legacy_only` | `legacy` | no |
| `dws aitable view update name` | `legacy_only` | `legacy` | no |
| `dws aitable view update row-height` | `legacy_only` | `legacy` | no |
| `dws aitable view update sort` | `legacy_only` | `legacy` | no |
| `dws aitable view update timebar` | `legacy_only` | `legacy` | no |
| `dws aitable view update visible-fields` | `legacy_only` | `legacy` | no |
| `dws aitable workflow` | `legacy_only` | `legacy` | no |
| `dws aitable workflow create` | `legacy_only` | `legacy` | no |
| `dws aitable workflow disable` | `legacy_only` | `legacy` | no |
| `dws aitable workflow edit-example` | `legacy_only` | `legacy` | no |
| `dws aitable workflow enable` | `legacy_only` | `legacy` | no |
| `dws aitable workflow get` | `legacy_only` | `legacy` | no |
| `dws aitable workflow list` | `legacy_only` | `legacy` | no |
| `dws aitable workflow update` | `legacy_only` | `legacy` | no |
| `dws api` | `legacy_only` | `legacy` | no |
| `dws attendance` | `legacy_only` | `legacy` | no |
| `dws attendance +boss-check` | `legacy_only` | `legacy` | no |
| `dws attendance +check-record` | `legacy_only` | `legacy` | no |
| `dws attendance +check-result` | `legacy_only` | `legacy` | no |
| `dws attendance +create-class` | `legacy_only` | `legacy` | no |
| `dws attendance +create-group` | `legacy_only` | `legacy` | no |
| `dws attendance +get-adjustment-rule` | `legacy_only` | `legacy` | no |
| `dws attendance +get-approve-template` | `legacy_only` | `legacy` | no |
| `dws attendance +get-checkin-record` | `legacy_only` | `legacy` | no |
| `dws attendance +get-class` | `legacy_only` | `legacy` | no |
| `dws attendance +get-global-setting` | `legacy_only` | `legacy` | no |
| `dws attendance +get-group` | `legacy_only` | `legacy` | no |
| `dws attendance +get-group-filtered` | `legacy_only` | `legacy` | no |
| `dws attendance +get-leave-balance` | `legacy_only` | `legacy` | no |
| `dws attendance +get-leave-records` | `legacy_only` | `legacy` | no |
| `dws attendance +get-overtime-rule` | `legacy_only` | `legacy` | no |
| `dws attendance +get-schedule` | `legacy_only` | `legacy` | no |
| `dws attendance +get-self-setting` | `legacy_only` | `legacy` | no |
| `dws attendance +get-summary` | `legacy_only` | `legacy` | no |
| `dws attendance +import-schedule` | `legacy_only` | `legacy` | no |
| `dws attendance +list-approve` | `legacy_only` | `legacy` | no |
| `dws attendance +list-leave-types` | `legacy_only` | `legacy` | no |
| `dws attendance +list-report-columns` | `legacy_only` | `legacy` | no |
| `dws attendance +my-attendance` | `legacy_only` | `legacy` | no |
| `dws attendance +query-report-data` | `legacy_only` | `legacy` | no |
| `dws attendance +query-report-leave` | `legacy_only` | `legacy` | no |
| `dws attendance +save-leave-balance` | `legacy_only` | `legacy` | no |
| `dws attendance +search-adjustment-rule` | `legacy_only` | `legacy` | no |
| `dws attendance +search-class` | `legacy_only` | `legacy` | no |
| `dws attendance +search-group` | `legacy_only` | `legacy` | no |
| `dws attendance +search-overtime-rule` | `legacy_only` | `legacy` | no |
| `dws attendance +this-month` | `legacy_only` | `legacy` | no |
| `dws attendance +update-class` | `legacy_only` | `legacy` | no |
| `dws attendance +update-group` | `legacy_only` | `legacy` | no |
| `dws attendance +update-group-members` | `legacy_only` | `legacy` | no |
| `dws attendance +update-leave-type` | `legacy_only` | `legacy` | no |
| `dws attendance adjustment` | `legacy_only` | `legacy` | no |
| `dws attendance adjustment get` | `legacy_only` | `legacy` | no |
| `dws attendance adjustment search` | `legacy_only` | `legacy` | no |
| `dws attendance approve` | `legacy_only` | `legacy` | no |
| `dws attendance approve list` | `legacy_only` | `legacy` | no |
| `dws attendance approve templates` | `legacy_only` | `legacy` | no |
| `dws attendance boss-check` | `legacy_only` | `legacy` | no |
| `dws attendance check` | `legacy_only` | `legacy` | no |
| `dws attendance check record` | `legacy_only` | `legacy` | no |
| `dws attendance check result` | `legacy_only` | `legacy` | no |
| `dws attendance checkin` | `legacy_only` | `legacy` | no |
| `dws attendance checkin records` | `legacy_only` | `legacy` | no |
| `dws attendance class` | `legacy_only` | `legacy` | no |
| `dws attendance class create` | `legacy_only` | `legacy` | no |
| `dws attendance class get` | `legacy_only` | `legacy` | no |
| `dws attendance class search` | `legacy_only` | `legacy` | no |
| `dws attendance class update` | `legacy_only` | `legacy` | no |
| `dws attendance globalsetting` | `legacy_only` | `legacy` | no |
| `dws attendance globalsetting get` | `legacy_only` | `legacy` | no |
| `dws attendance globalsetting save` | `legacy_only` | `legacy` | no |
| `dws attendance group` | `legacy_only` | `legacy` | no |
| `dws attendance group create` | `legacy_only` | `legacy` | no |
| `dws attendance group filtered-get` | `legacy_only` | `legacy` | no |
| `dws attendance group get` | `legacy_only` | `legacy` | no |
| `dws attendance group search` | `legacy_only` | `legacy` | no |
| `dws attendance group update` | `legacy_only` | `legacy` | no |
| `dws attendance group update-members` | `legacy_only` | `legacy` | no |
| `dws attendance overtime` | `legacy_only` | `legacy` | no |
| `dws attendance overtime get` | `legacy_only` | `legacy` | no |
| `dws attendance overtime search` | `legacy_only` | `legacy` | no |
| `dws attendance record` | `legacy_only` | `legacy` | no |
| `dws attendance record get` | `legacy_only` | `legacy` | no |
| `dws attendance report` | `legacy_only` | `legacy` | no |
| `dws attendance report columns` | `legacy_only` | `legacy` | no |
| `dws attendance report query-data` | `legacy_only` | `legacy` | no |
| `dws attendance report query-leave` | `legacy_only` | `legacy` | no |
| `dws attendance rules` | `legacy_only` | `legacy` | no |
| `dws attendance schedule` | `legacy_only` | `legacy` | no |
| `dws attendance schedule get` | `legacy_only` | `legacy` | no |
| `dws attendance schedule import` | `legacy_only` | `legacy` | no |
| `dws attendance selfsetting` | `legacy_only` | `legacy` | no |
| `dws attendance selfsetting get` | `legacy_only` | `legacy` | no |
| `dws attendance selfsetting save` | `legacy_only` | `legacy` | no |
| `dws attendance shift` | `legacy_only` | `legacy` | no |
| `dws attendance shift list` | `legacy_only` | `legacy` | no |
| `dws attendance summary` | `legacy_only` | `legacy` | no |
| `dws attendance vacation` | `legacy_only` | `legacy` | no |
| `dws attendance vacation balance` | `legacy_only` | `legacy` | no |
| `dws attendance vacation records` | `legacy_only` | `legacy` | no |
| `dws attendance vacation save-balance` | `legacy_only` | `legacy` | no |
| `dws attendance vacation types` | `legacy_only` | `legacy` | no |
| `dws attendance vacation update-type` | `legacy_only` | `legacy` | no |
| `dws audit export` | `legacy_only` | `legacy` | no |
| `dws audit tail` | `legacy_only` | `legacy` | no |
| `dws audit verify` | `legacy_only` | `legacy` | no |
| `dws auth` | `legacy_only` | `legacy` | no |
| `dws auth exchange` | `legacy_only` | `legacy` | yes |
| `dws auth export` | `legacy_only` | `legacy` | no |
| `dws auth import` | `legacy_only` | `legacy` | no |
| `dws auth login` | `legacy_only` | `legacy` | no |
| `dws auth logout` | `legacy_only` | `legacy` | no |
| `dws auth migrate-keychain` | `legacy_only` | `legacy` | no |
| `dws auth reset` | `legacy_only` | `legacy` | no |
| `dws auth status` | `legacy_only` | `legacy` | no |
| `dws cache` | `legacy_only` | `legacy` | no |
| `dws cache clean` | `legacy_only` | `legacy` | no |
| `dws cache refresh` | `legacy_only` | `legacy` | no |
| `dws cache status` | `legacy_only` | `legacy` | no |
| `dws calendar` | `legacy_only` | `legacy` | no |
| `dws calendar +agenda` | `unified_active` | `unified` | no |
| `dws calendar +attendee-list` | `unified_active` | `unified` | no |
| `dws calendar +book` | `legacy_only` | `legacy` | no |
| `dws calendar +book-list` | `unified_active` | `unified` | no |
| `dws calendar +book-search` | `unified_active` | `unified` | no |
| `dws calendar +cancel-event` | `legacy_only` | `legacy` | no |
| `dws calendar +conflicts` | `unified_active` | `unified` | no |
| `dws calendar +find-room` | `unified_active` | `unified` | no |
| `dws calendar +free` | `unified_active` | `unified` | no |
| `dws calendar +free-slots` | `unified_active` | `unified` | no |
| `dws calendar +freebusy` | `legacy_only` | `legacy` | no |
| `dws calendar +invite` | `legacy_only` | `legacy` | no |
| `dws calendar +my-free` | `unified_active` | `unified` | no |
| `dws calendar +next-event` | `unified_active` | `unified` | no |
| `dws calendar +reschedule` | `legacy_only` | `legacy` | no |
| `dws calendar +respond-event` | `legacy_only` | `legacy` | yes |
| `dws calendar +room-find` | `legacy_only` | `legacy` | yes |
| `dws calendar +room-groups` | `unified_active` | `unified` | no |
| `dws calendar +room-search` | `unified_active` | `unified` | no |
| `dws calendar +suggest-time` | `legacy_only` | `legacy` | no |
| `dws calendar +today` | `unified_active` | `unified` | no |
| `dws calendar +tomorrow` | `unified_active` | `unified` | no |
| `dws calendar +week` | `unified_active` | `unified` | no |
| `dws calendar acl` | `legacy_only` | `legacy` | no |
| `dws calendar acl add` | `legacy_only` | `legacy` | no |
| `dws calendar acl delete` | `legacy_only` | `legacy` | no |
| `dws calendar acl list` | `legacy_only` | `legacy` | no |
| `dws calendar add` | `legacy_only` | `legacy` | yes |
| `dws calendar attachment` | `legacy_only` | `legacy` | no |
| `dws calendar attachment add` | `legacy_only` | `legacy` | no |
| `dws calendar attendee` | `legacy_only` | `legacy` | no |
| `dws calendar attendee add` | `legacy_only` | `legacy` | no |
| `dws calendar attendee delete` | `legacy_only` | `legacy` | no |
| `dws calendar attendee list` | `legacy_only` | `legacy` | no |
| `dws calendar book` | `legacy_only` | `legacy` | no |
| `dws calendar book get` | `legacy_only` | `legacy` | no |
| `dws calendar book list` | `legacy_only` | `legacy` | no |
| `dws calendar book search` | `legacy_only` | `legacy` | no |
| `dws calendar book update` | `legacy_only` | `legacy` | no |
| `dws calendar busy` | `legacy_only` | `legacy` | no |
| `dws calendar busy search` | `legacy_only` | `legacy` | no |
| `dws calendar create` | `legacy_only` | `legacy` | yes |
| `dws calendar delete` | `legacy_only` | `legacy` | yes |
| `dws calendar event` | `legacy_only` | `legacy` | no |
| `dws calendar event create` | `legacy_only` | `legacy` | no |
| `dws calendar event delete` | `legacy_only` | `legacy` | no |
| `dws calendar event get` | `legacy_only` | `legacy` | no |
| `dws calendar event instances` | `legacy_only` | `legacy` | no |
| `dws calendar event list` | `legacy_only` | `legacy` | no |
| `dws calendar event respond` | `legacy_only` | `legacy` | no |
| `dws calendar event suggest` | `legacy_only` | `legacy` | no |
| `dws calendar event update` | `legacy_only` | `legacy` | no |
| `dws calendar get` | `legacy_only` | `legacy` | yes |
| `dws calendar list` | `legacy_only` | `legacy` | yes |
| `dws calendar list-groups` | `legacy_only` | `legacy` | yes |
| `dws calendar respond` | `legacy_only` | `legacy` | yes |
| `dws calendar room` | `legacy_only` | `legacy` | no |
| `dws calendar room add` | `legacy_only` | `legacy` | no |
| `dws calendar room delete` | `legacy_only` | `legacy` | no |
| `dws calendar room list-groups` | `legacy_only` | `legacy` | no |
| `dws calendar room search` | `legacy_only` | `legacy` | no |
| `dws calendar search` | `legacy_only` | `legacy` | yes |
| `dws calendar suggest` | `legacy_only` | `legacy` | yes |
| `dws calendar today` | `legacy_only` | `legacy` | yes |
| `dws calendar update` | `legacy_only` | `legacy` | yes |
| `dws catalog` | `legacy_only` | `legacy` | yes |
| `dws chat` | `legacy_only` | `legacy` | no |
| `dws chat +at-me` | `unified_active` | `unified` | no |
| `dws chat +bot-find` | `legacy_only` | `legacy` | no |
| `dws chat +bot-search` | `legacy_only` | `legacy` | no |
| `dws chat +broadcast` | `legacy_only` | `legacy` | no |
| `dws chat +category-add-conversation` | `legacy_only` | `legacy` | no |
| `dws chat +category-create` | `legacy_only` | `legacy` | no |
| `dws chat +category-delete` | `legacy_only` | `legacy` | no |
| `dws chat +category-list` | `legacy_only` | `legacy` | no |
| `dws chat +category-list-conversations` | `legacy_only` | `legacy` | no |
| `dws chat +category-remove-conversation` | `legacy_only` | `legacy` | no |
| `dws chat +category-rename` | `legacy_only` | `legacy` | no |
| `dws chat +chat-add-bot` | `legacy_only` | `legacy` | no |
| `dws chat +chat-audit-join` | `legacy_only` | `legacy` | no |
| `dws chat +chat-bots` | `legacy_only` | `legacy` | no |
| `dws chat +chat-create` | `legacy_only` | `legacy` | no |
| `dws chat +chat-dismiss` | `legacy_only` | `legacy` | no |
| `dws chat +chat-get-by-id` | `legacy_only` | `legacy` | no |
| `dws chat +chat-invite-url` | `legacy_only` | `legacy` | no |
| `dws chat +chat-list` | `legacy_only` | `legacy` | no |
| `dws chat +chat-list-all` | `legacy_only` | `legacy` | no |
| `dws chat +chat-list-join-requests` | `legacy_only` | `legacy` | no |
| `dws chat +chat-list-mine` | `legacy_only` | `legacy` | no |
| `dws chat +chat-members-get` | `legacy_only` | `legacy` | no |
| `dws chat +chat-members-list` | `legacy_only` | `legacy` | no |
| `dws chat +chat-messages` | `unified_active` | `unified` | no |
| `dws chat +chat-mute` | `legacy_only` | `legacy` | no |
| `dws chat +chat-mute-member` | `legacy_only` | `legacy` | no |
| `dws chat +chat-quit` | `legacy_only` | `legacy` | no |
| `dws chat +chat-remove-bot` | `legacy_only` | `legacy` | no |
| `dws chat +chat-role-add` | `legacy_only` | `legacy` | no |
| `dws chat +chat-role-list` | `legacy_only` | `legacy` | no |
| `dws chat +chat-role-query-user` | `legacy_only` | `legacy` | no |
| `dws chat +chat-role-remove` | `legacy_only` | `legacy` | no |
| `dws chat +chat-role-remove-user` | `legacy_only` | `legacy` | no |
| `dws chat +chat-role-set-user` | `legacy_only` | `legacy` | no |
| `dws chat +chat-role-update` | `legacy_only` | `legacy` | no |
| `dws chat +chat-search` | `unified_active` | `unified` | no |
| `dws chat +chat-set-admin` | `legacy_only` | `legacy` | no |
| `dws chat +chat-set-history` | `legacy_only` | `legacy` | no |
| `dws chat +chat-transfer-owner` | `legacy_only` | `legacy` | no |
| `dws chat +chat-update` | `legacy_only` | `legacy` | no |
| `dws chat +chat-update-alias` | `legacy_only` | `legacy` | no |
| `dws chat +chat-update-icon` | `legacy_only` | `legacy` | no |
| `dws chat +chat-update-nick` | `legacy_only` | `legacy` | no |
| `dws chat +chat-update-settings` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-clear-all-red-point` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-clear-messages` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-clear-red-point` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-hide` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-info` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-list` | `unified_active` | `unified` | no |
| `dws chat +conversation-list-top` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-mark-read` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-mark-unread` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-mute` | `legacy_only` | `legacy` | no |
| `dws chat +conversation-mute-at-all` | `legacy_only` | `legacy` | yes |
| `dws chat +conversation-mute-red-envelope` | `legacy_only` | `legacy` | yes |
| `dws chat +conversation-set-top` | `legacy_only` | `legacy` | no |
| `dws chat +dm` | `legacy_only` | `legacy` | no |
| `dws chat +feed-group-query-item` | `legacy_only` | `legacy` | no |
| `dws chat +flag-cancel` | `legacy_only` | `legacy` | no |
| `dws chat +flag-create` | `legacy_only` | `legacy` | no |
| `dws chat +flag-list` | `unified_active` | `unified` | no |
| `dws chat +group-members` | `legacy_only` | `legacy` | no |
| `dws chat +messages-add-emoji` | `legacy_only` | `legacy` | no |
| `dws chat +messages-add-text-emotion` | `legacy_only` | `legacy` | no |
| `dws chat +messages-batch-recall-by-bot` | `legacy_only` | `legacy` | no |
| `dws chat +messages-batch-send-by-bot` | `legacy_only` | `legacy` | no |
| `dws chat +messages-combine-forward` | `legacy_only` | `legacy` | no |
| `dws chat +messages-create-text-emotion` | `legacy_only` | `legacy` | no |
| `dws chat +messages-forward` | `legacy_only` | `legacy` | no |
| `dws chat +messages-forward-topic` | `legacy_only` | `legacy` | no |
| `dws chat +messages-list` | `legacy_only` | `legacy` | no |
| `dws chat +messages-list-direct` | `legacy_only` | `legacy` | no |
| `dws chat +messages-list-pin` | `legacy_only` | `legacy` | no |
| `dws chat +messages-list-unread-conversations` | `legacy_only` | `legacy` | no |
| `dws chat +messages-mget` | `legacy_only` | `legacy` | no |
| `dws chat +messages-query-send-status` | `legacy_only` | `legacy` | no |
| `dws chat +messages-read-status` | `legacy_only` | `legacy` | no |
| `dws chat +messages-recall` | `legacy_only` | `legacy` | no |
| `dws chat +messages-recall-by-bot` | `legacy_only` | `legacy` | no |
| `dws chat +messages-remove-emoji` | `legacy_only` | `legacy` | no |
| `dws chat +messages-remove-text-emotion` | `legacy_only` | `legacy` | no |
| `dws chat +messages-reply` | `legacy_only` | `legacy` | no |
| `dws chat +messages-resource-download` | `legacy_only` | `legacy` | no |
| `dws chat +messages-resource-url` | `legacy_only` | `legacy` | no |
| `dws chat +messages-send` | `legacy_only` | `legacy` | no |
| `dws chat +messages-send-by-bot` | `legacy_only` | `legacy` | no |
| `dws chat +messages-send-by-webhook` | `legacy_only` | `legacy` | no |
| `dws chat +messages-send-card` | `legacy_only` | `legacy` | no |
| `dws chat +messages-set-pin` | `legacy_only` | `legacy` | no |
| `dws chat +messages-set-top` | `legacy_only` | `legacy` | no |
| `dws chat +messages-unset-pin` | `legacy_only` | `legacy` | no |
| `dws chat +messages-unset-top` | `legacy_only` | `legacy` | no |
| `dws chat +messages-update-card` | `legacy_only` | `legacy` | no |
| `dws chat +my-groups` | `legacy_only` | `legacy` | no |
| `dws chat +search-msg` | `unified_active` | `unified` | no |
| `dws chat +send-to-group` | `legacy_only` | `legacy` | no |
| `dws chat +thread-replies` | `unified_active` | `unified` | no |
| `dws chat +unread-chats` | `legacy_only` | `legacy` | no |
| `dws chat bot` | `legacy_only` | `legacy` | no |
| `dws chat bot find` | `legacy_only` | `legacy` | no |
| `dws chat bot search` | `legacy_only` | `legacy` | no |
| `dws chat category` | `legacy_only` | `legacy` | no |
| `dws chat category add-conv` | `legacy_only` | `legacy` | no |
| `dws chat category batch-info` | `legacy_only` | `legacy` | no |
| `dws chat category create` | `legacy_only` | `legacy` | no |
| `dws chat category create-smart` | `legacy_only` | `legacy` | no |
| `dws chat category delete` | `legacy_only` | `legacy` | no |
| `dws chat category list` | `legacy_only` | `legacy` | no |
| `dws chat category list-by-conv` | `legacy_only` | `legacy` | no |
| `dws chat category list-conversations` | `legacy_only` | `legacy` | no |
| `dws chat category remove-conv` | `legacy_only` | `legacy` | no |
| `dws chat category rename` | `legacy_only` | `legacy` | no |
| `dws chat chmod` | `legacy_only` | `legacy` | no |
| `dws chat clear-all-red-point` | `legacy_only` | `legacy` | no |
| `dws chat clear-messages` | `legacy_only` | `legacy` | no |
| `dws chat clear-red-point` | `legacy_only` | `legacy` | no |
| `dws chat conversation-info` | `legacy_only` | `legacy` | no |
| `dws chat data-auth` | `legacy_only` | `legacy` | no |
| `dws chat data-auth cross-org` | `legacy_only` | `legacy` | no |
| `dws chat file` | `legacy_only` | `legacy` | yes |
| `dws chat file upload` | `legacy_only` | `legacy` | yes |
| `dws chat group` | `legacy_only` | `legacy` | no |
| `dws chat group audit-join-validation` | `legacy_only` | `legacy` | no |
| `dws chat group bots` | `legacy_only` | `legacy` | no |
| `dws chat group create` | `legacy_only` | `legacy` | no |
| `dws chat group dismiss` | `legacy_only` | `legacy` | no |
| `dws chat group get-by-group-id` | `legacy_only` | `legacy` | no |
| `dws chat group get-mute-config` | `legacy_only` | `legacy` | no |
| `dws chat group invite-url` | `legacy_only` | `legacy` | no |
| `dws chat group list-all` | `legacy_only` | `legacy` | no |
| `dws chat group list-join-validations` | `legacy_only` | `legacy` | no |
| `dws chat group list-my-groups` | `legacy_only` | `legacy` | no |
| `dws chat group members` | `legacy_only` | `legacy` | no |
| `dws chat group members add` | `legacy_only` | `legacy` | no |
| `dws chat group members add-bot` | `legacy_only` | `legacy` | no |
| `dws chat group members list-by-ids` | `legacy_only` | `legacy` | no |
| `dws chat group members remove` | `legacy_only` | `legacy` | no |
| `dws chat group members remove-bot` | `legacy_only` | `legacy` | no |
| `dws chat group notice` | `legacy_only` | `legacy` | no |
| `dws chat group notice create` | `legacy_only` | `legacy` | no |
| `dws chat group notice edit` | `legacy_only` | `legacy` | no |
| `dws chat group notice get` | `legacy_only` | `legacy` | no |
| `dws chat group notice list` | `legacy_only` | `legacy` | no |
| `dws chat group quit` | `legacy_only` | `legacy` | no |
| `dws chat group rename` | `legacy_only` | `legacy` | no |
| `dws chat group search` | `legacy_only` | `legacy` | yes |
| `dws chat group set-admin` | `legacy_only` | `legacy` | no |
| `dws chat group set-history` | `legacy_only` | `legacy` | no |
| `dws chat group share-invite` | `legacy_only` | `legacy` | no |
| `dws chat group transfer-owner` | `legacy_only` | `legacy` | no |
| `dws chat group update-alias` | `legacy_only` | `legacy` | no |
| `dws chat group update-icon` | `legacy_only` | `legacy` | no |
| `dws chat group update-nick` | `legacy_only` | `legacy` | no |
| `dws chat group update-settings` | `legacy_only` | `legacy` | no |
| `dws chat group upgrade-to-external` | `legacy_only` | `legacy` | no |
| `dws chat group user-settings` | `legacy_only` | `legacy` | no |
| `dws chat group user-settings query` | `legacy_only` | `legacy` | no |
| `dws chat group user-settings set` | `legacy_only` | `legacy` | no |
| `dws chat group-mute` | `legacy_only` | `legacy` | no |
| `dws chat group-mute-member` | `legacy_only` | `legacy` | no |
| `dws chat group-role` | `legacy_only` | `legacy` | no |
| `dws chat group-role add` | `legacy_only` | `legacy` | no |
| `dws chat group-role list` | `legacy_only` | `legacy` | no |
| `dws chat group-role query-user` | `legacy_only` | `legacy` | no |
| `dws chat group-role remove` | `legacy_only` | `legacy` | no |
| `dws chat group-role remove-user` | `legacy_only` | `legacy` | no |
| `dws chat group-role set-user` | `legacy_only` | `legacy` | no |
| `dws chat group-role update` | `legacy_only` | `legacy` | no |
| `dws chat hide` | `legacy_only` | `legacy` | no |
| `dws chat history` | `legacy_only` | `legacy` | yes |
| `dws chat list-all-conversations` | `legacy_only` | `legacy` | no |
| `dws chat list-top-conversations` | `legacy_only` | `legacy` | no |
| `dws chat mark-read` | `legacy_only` | `legacy` | no |
| `dws chat mark-unread` | `legacy_only` | `legacy` | no |
| `dws chat media` | `legacy_only` | `legacy` | no |
| `dws chat media upload` | `legacy_only` | `legacy` | no |
| `dws chat message` | `legacy_only` | `legacy` | no |
| `dws chat message add-emoji` | `legacy_only` | `legacy` | no |
| `dws chat message add-favorite` | `legacy_only` | `legacy` | no |
| `dws chat message add-text-emotion` | `legacy_only` | `legacy` | no |
| `dws chat message combine-forward` | `legacy_only` | `legacy` | no |
| `dws chat message create-text-emotion` | `legacy_only` | `legacy` | no |
| `dws chat message download-media` | `legacy_only` | `legacy` | no |
| `dws chat message edit` | `legacy_only` | `legacy` | no |
| `dws chat message forward` | `legacy_only` | `legacy` | no |
| `dws chat message forward-topic` | `legacy_only` | `legacy` | no |
| `dws chat message list` | `legacy_only` | `legacy` | no |
| `dws chat message list-all` | `legacy_only` | `legacy` | no |
| `dws chat message list-by-ids` | `legacy_only` | `legacy` | no |
| `dws chat message list-by-sender` | `legacy_only` | `legacy` | no |
| `dws chat message list-direct` | `legacy_only` | `legacy` | yes |
| `dws chat message list-emotion-replies` | `legacy_only` | `legacy` | no |
| `dws chat message list-favorites` | `legacy_only` | `legacy` | no |
| `dws chat message list-focused` | `legacy_only` | `legacy` | no |
| `dws chat message list-mentions` | `legacy_only` | `legacy` | no |
| `dws chat message list-pin-msg` | `legacy_only` | `legacy` | no |
| `dws chat message list-topic-replies` | `legacy_only` | `legacy` | no |
| `dws chat message list-unread-conversations` | `legacy_only` | `legacy` | no |
| `dws chat message query-send-status` | `legacy_only` | `legacy` | no |
| `dws chat message read-status` | `legacy_only` | `legacy` | no |
| `dws chat message recall` | `legacy_only` | `legacy` | no |
| `dws chat message recall-by-bot` | `legacy_only` | `legacy` | no |
| `dws chat message remove-emoji` | `legacy_only` | `legacy` | no |
| `dws chat message remove-favorite` | `legacy_only` | `legacy` | no |
| `dws chat message remove-text-emotion` | `legacy_only` | `legacy` | no |
| `dws chat message reply` | `legacy_only` | `legacy` | no |
| `dws chat message search` | `legacy_only` | `legacy` | no |
| `dws chat message search-advanced` | `legacy_only` | `legacy` | no |
| `dws chat message search-common` | `legacy_only` | `legacy` | yes |
| `dws chat message send` | `legacy_only` | `legacy` | no |
| `dws chat message send-by-bot` | `legacy_only` | `legacy` | no |
| `dws chat message send-by-webhook` | `legacy_only` | `legacy` | no |
| `dws chat message send-card` | `legacy_only` | `legacy` | no |
| `dws chat message set-pin-msg` | `legacy_only` | `legacy` | no |
| `dws chat message set-top-msg` | `legacy_only` | `legacy` | no |
| `dws chat message unset-pin-msg` | `legacy_only` | `legacy` | no |
| `dws chat message unset-top-msg` | `legacy_only` | `legacy` | no |
| `dws chat message update-card` | `legacy_only` | `legacy` | no |
| `dws chat message update-text-emotion` | `legacy_only` | `legacy` | no |
| `dws chat mute` | `legacy_only` | `legacy` | no |
| `dws chat mute-at-all` | `legacy_only` | `legacy` | no |
| `dws chat mute-red-envelope` | `legacy_only` | `legacy` | no |
| `dws chat search` | `legacy_only` | `legacy` | no |
| `dws chat search-common` | `legacy_only` | `legacy` | no |
| `dws chat send` | `legacy_only` | `legacy` | yes |
| `dws chat set-top` | `legacy_only` | `legacy` | no |
| `dws chat text` | `legacy_only` | `legacy` | no |
| `dws chat text translate` | `legacy_only` | `legacy` | no |
| `dws chat toolbar` | `legacy_only` | `legacy` | no |
| `dws chat toolbar add` | `legacy_only` | `legacy` | no |
| `dws chat toolbar create-custom` | `legacy_only` | `legacy` | no |
| `dws chat toolbar hide` | `legacy_only` | `legacy` | no |
| `dws chat toolbar list` | `legacy_only` | `legacy` | no |
| `dws chat toolbar remove-custom` | `legacy_only` | `legacy` | no |
| `dws chat toolbar sort` | `legacy_only` | `legacy` | no |
| `dws chat toolbar update-custom` | `legacy_only` | `legacy` | no |
| `dws completion` | `legacy_only` | `legacy` | no |
| `dws conference` | `legacy_only` | `legacy` | yes |
| `dws conference meeting` | `legacy_only` | `legacy` | no |
| `dws conference meeting reserve` | `legacy_only` | `legacy` | no |
| `dws conference member` | `legacy_only` | `legacy` | no |
| `dws conference member invite` | `legacy_only` | `legacy` | no |
| `dws config` | `legacy_only` | `legacy` | no |
| `dws config list` | `legacy_only` | `legacy` | no |
| `dws contact` | `legacy_only` | `legacy` | no |
| `dws contact +by-mobile` | `legacy_only` | `legacy` | no |
| `dws contact +dept-members` | `legacy_only` | `legacy` | no |
| `dws contact +get-roster` | `legacy_only` | `legacy` | yes |
| `dws contact +list-dept-members` | `unified_active` | `unified` | no |
| `dws contact +list-followings` | `unified_active` | `unified` | no |
| `dws contact +list-role-members` | `unified_active` | `unified` | no |
| `dws contact +list-roles` | `unified_active` | `unified` | no |
| `dws contact +list-roster-fields` | `legacy_only` | `legacy` | yes |
| `dws contact +list-sub-depts` | `unified_active` | `unified` | no |
| `dws contact +lookup` | `legacy_only` | `legacy` | no |
| `dws contact +me` | `legacy_only` | `legacy` | no |
| `dws contact +org` | `legacy_only` | `legacy` | no |
| `dws contact +resolve-dept` | `legacy_only` | `legacy` | no |
| `dws contact +search-mobile` | `unified_active` | `unified` | no |
| `dws contact +search-user` | `unified_active` | `unified` | no |
| `dws contact +team` | `legacy_only` | `legacy` | no |
| `dws contact account` | `legacy_only` | `legacy` | no |
| `dws contact account create` | `legacy_only` | `legacy` | no |
| `dws contact account update` | `legacy_only` | `legacy` | no |
| `dws contact current-user` | `legacy_only` | `legacy` | yes |
| `dws contact department` | `legacy_only` | `legacy` | yes |
| `dws contact dept` | `legacy_only` | `legacy` | no |
| `dws contact dept create` | `legacy_only` | `legacy` | no |
| `dws contact dept detail` | `legacy_only` | `legacy` | yes |
| `dws contact dept get-info` | `legacy_only` | `legacy` | no |
| `dws contact dept info` | `legacy_only` | `legacy` | yes |
| `dws contact dept list` | `legacy_only` | `legacy` | yes |
| `dws contact dept list-children` | `legacy_only` | `legacy` | no |
| `dws contact dept list-members` | `legacy_only` | `legacy` | no |
| `dws contact dept search` | `legacy_only` | `legacy` | no |
| `dws contact dept update` | `legacy_only` | `legacy` | no |
| `dws contact find` | `legacy_only` | `legacy` | yes |
| `dws contact get` | `legacy_only` | `legacy` | yes |
| `dws contact get-self` | `legacy_only` | `legacy` | yes |
| `dws contact label` | `legacy_only` | `legacy` | no |
| `dws contact label detail` | `legacy_only` | `legacy` | yes |
| `dws contact label find` | `legacy_only` | `legacy` | yes |
| `dws contact label get` | `legacy_only` | `legacy` | no |
| `dws contact label info` | `legacy_only` | `legacy` | yes |
| `dws contact label list` | `legacy_only` | `legacy` | no |
| `dws contact label list-all` | `legacy_only` | `legacy` | yes |
| `dws contact label list-members` | `legacy_only` | `legacy` | no |
| `dws contact label search` | `legacy_only` | `legacy` | yes |
| `dws contact list` | `legacy_only` | `legacy` | yes |
| `dws contact me` | `legacy_only` | `legacy` | yes |
| `dws contact org` | `legacy_only` | `legacy` | no |
| `dws contact org create` | `legacy_only` | `legacy` | no |
| `dws contact relation` | `legacy_only` | `legacy` | no |
| `dws contact relation list-my-followings` | `legacy_only` | `legacy` | no |
| `dws contact search` | `legacy_only` | `legacy` | yes |
| `dws contact self` | `legacy_only` | `legacy` | yes |
| `dws contact user` | `legacy_only` | `legacy` | no |
| `dws contact user detail` | `legacy_only` | `legacy` | yes |
| `dws contact user dismission` | `legacy_only` | `legacy` | no |
| `dws contact user dismission search` | `legacy_only` | `legacy` | no |
| `dws contact user find` | `legacy_only` | `legacy` | yes |
| `dws contact user get` | `legacy_only` | `legacy` | no |
| `dws contact user get-info` | `legacy_only` | `legacy` | yes |
| `dws contact user get-self` | `legacy_only` | `legacy` | no |
| `dws contact user info` | `legacy_only` | `legacy` | yes |
| `dws contact user invite` | `legacy_only` | `legacy` | no |
| `dws contact user list` | `legacy_only` | `legacy` | yes |
| `dws contact user profile` | `legacy_only` | `legacy` | no |
| `dws contact user profile fields` | `legacy_only` | `legacy` | no |
| `dws contact user profile get` | `legacy_only` | `legacy` | no |
| `dws contact user search` | `legacy_only` | `legacy` | no |
| `dws contact user search-mobile` | `legacy_only` | `legacy` | no |
| `dws contact user update` | `legacy_only` | `legacy` | no |
| `dws contact user update-ownness` | `legacy_only` | `legacy` | no |
| `dws contact user update-self` | `legacy_only` | `legacy` | no |
| `dws contact user-self` | `legacy_only` | `legacy` | yes |
| `dws contact whoami` | `legacy_only` | `legacy` | yes |
| `dws dev` | `legacy_only` | `legacy` | no |
| `dws dev app` | `legacy_only` | `legacy` | no |
| `dws dev app create` | `unified_active` | `unified` | no |
| `dws dev app credentials` | `legacy_only` | `legacy` | no |
| `dws dev app credentials get` | `unified_active` | `unified` | no |
| `dws dev app delete` | `unified_active` | `unified` | no |
| `dws dev app disable` | `unified_active` | `unified` | no |
| `dws dev app enable` | `unified_active` | `unified` | no |
| `dws dev app event` | `legacy_only` | `legacy` | no |
| `dws dev app event list` | `unified_active` | `unified` | no |
| `dws dev app event subscribe` | `unified_active` | `unified` | no |
| `dws dev app event unsubscribe` | `unified_active` | `unified` | no |
| `dws dev app get` | `unified_active` | `unified` | no |
| `dws dev app list` | `unified_active` | `unified` | no |
| `dws dev app member` | `legacy_only` | `legacy` | no |
| `dws dev app member add` | `unified_active` | `unified` | no |
| `dws dev app member list` | `unified_active` | `unified` | no |
| `dws dev app member remove` | `unified_active` | `unified` | no |
| `dws dev app permission` | `legacy_only` | `legacy` | no |
| `dws dev app permission add` | `unified_active` | `unified` | no |
| `dws dev app permission list` | `unified_active` | `unified` | no |
| `dws dev app permission remove` | `unified_active` | `unified` | no |
| `dws dev app robot` | `legacy_only` | `legacy` | no |
| `dws dev app robot config` | `unified_active` | `unified` | no |
| `dws dev app robot disable` | `unified_active` | `unified` | no |
| `dws dev app robot enable` | `unified_active` | `unified` | no |
| `dws dev app robot get` | `unified_active` | `unified` | no |
| `dws dev app robot result` | `unified_active` | `unified` | no |
| `dws dev app robot submit` | `unified_active` | `unified` | no |
| `dws dev app security` | `legacy_only` | `legacy` | no |
| `dws dev app security config` | `unified_active` | `unified` | no |
| `dws dev app update` | `unified_active` | `unified` | no |
| `dws dev app version` | `legacy_only` | `legacy` | no |
| `dws dev app version check-approval` | `unified_active` | `unified` | no |
| `dws dev app version create` | `unified_active` | `unified` | no |
| `dws dev app version get` | `unified_active` | `unified` | no |
| `dws dev app version list` | `unified_active` | `unified` | no |
| `dws dev app version publish` | `unified_active` | `unified` | no |
| `dws dev app version status` | `unified_active` | `unified` | no |
| `dws dev app webapp` | `legacy_only` | `legacy` | no |
| `dws dev app webapp config` | `unified_active` | `unified` | no |
| `dws dev app webapp get` | `unified_active` | `unified` | no |
| `dws dev connect` | `legacy_only` | `legacy` | no |
| `dws dev connect list` | `unified_active` | `unified` | no |
| `dws dev connect restart` | `unified_active` | `unified` | no |
| `dws dev connect status` | `unified_active` | `unified` | no |
| `dws dev connect stop` | `unified_active` | `unified` | no |
| `dws dev doc` | `legacy_only` | `legacy` | no |
| `dws dev doc search` | `unified_active` | `unified` | no |
| `dws devapp +create` | `dual_validate` | `legacy` | no |
| `dws devapp +credentials-get` | `unified_active` | `unified` | no |
| `dws devapp +delete` | `dual_validate` | `legacy` | no |
| `dws devapp +disable` | `dual_validate` | `legacy` | no |
| `dws devapp +enable` | `dual_validate` | `legacy` | no |
| `dws devapp +event-list` | `unified_active` | `unified` | no |
| `dws devapp +event-subscribe` | `dual_validate` | `legacy` | yes |
| `dws devapp +event-unsubscribe` | `dual_validate` | `legacy` | yes |
| `dws devapp +get` | `unified_active` | `unified` | no |
| `dws devapp +list` | `unified_active` | `unified` | no |
| `dws devapp +member-add` | `dual_validate` | `legacy` | no |
| `dws devapp +member-list` | `dual_validate` | `legacy` | no |
| `dws devapp +member-remove` | `dual_validate` | `legacy` | no |
| `dws devapp +permission-add` | `dual_validate` | `legacy` | yes |
| `dws devapp +permission-list` | `unified_active` | `unified` | no |
| `dws devapp +permission-remove` | `dual_validate` | `legacy` | yes |
| `dws devapp +robot-config` | `dual_validate` | `legacy` | yes |
| `dws devapp +robot-disable` | `dual_validate` | `legacy` | yes |
| `dws devapp +robot-enable` | `dual_validate` | `legacy` | yes |
| `dws devapp +robot-get` | `unified_active` | `unified` | no |
| `dws devapp +security-config` | `dual_validate` | `legacy` | yes |
| `dws devapp +update` | `dual_validate` | `legacy` | no |
| `dws devapp +version-check-approval` | `unified_active` | `unified` | no |
| `dws devapp +version-create` | `dual_validate` | `legacy` | yes |
| `dws devapp +version-get` | `unified_active` | `unified` | no |
| `dws devapp +version-list` | `unified_active` | `unified` | no |
| `dws devapp +version-publish` | `dual_validate` | `legacy` | yes |
| `dws devapp +version-status` | `unified_active` | `unified` | no |
| `dws devapp +webapp-config` | `dual_validate` | `legacy` | no |
| `dws devapp +webapp-get` | `unified_active` | `unified` | no |
| `dws devdoc` | `legacy_only` | `legacy` | no |
| `dws devdoc article` | `legacy_only` | `legacy` | no |
| `dws devdoc article search` | `legacy_only` | `legacy` | no |
| `dws devdoc search` | `legacy_only` | `legacy` | yes |
| `dws ding` | `legacy_only` | `legacy` | no |
| `dws ding +list` | `legacy_only` | `legacy` | no |
| `dws ding +recall-personal` | `legacy_only` | `legacy` | no |
| `dws ding +receiver-status` | `legacy_only` | `legacy` | no |
| `dws ding +send-by-message` | `legacy_only` | `legacy` | yes |
| `dws ding +send-personal` | `legacy_only` | `legacy` | no |
| `dws ding message` | `legacy_only` | `legacy` | no |
| `dws ding message list` | `legacy_only` | `legacy` | no |
| `dws ding message recall` | `legacy_only` | `legacy` | no |
| `dws ding message recall-personal` | `legacy_only` | `legacy` | no |
| `dws ding message receiver-status` | `legacy_only` | `legacy` | no |
| `dws ding message send` | `legacy_only` | `legacy` | no |
| `dws ding message send-by-message` | `legacy_only` | `legacy` | no |
| `dws ding message send-personal` | `legacy_only` | `legacy` | no |
| `dws doc` | `legacy_only` | `legacy` | no |
| `dws doc +access-change` | `legacy_only` | `legacy` | no |
| `dws doc +access-grant` | `legacy_only` | `legacy` | no |
| `dws doc +access-revoke` | `legacy_only` | `legacy` | no |
| `dws doc +background-delete` | `legacy_only` | `legacy` | no |
| `dws doc +background-update` | `legacy_only` | `legacy` | no |
| `dws doc +checkpoint-update` | `legacy_only` | `legacy` | no |
| `dws doc +comment-create` | `legacy_only` | `legacy` | no |
| `dws doc +comment-create-inline` | `legacy_only` | `legacy` | yes |
| `dws doc +comment-delete` | `legacy_only` | `legacy` | no |
| `dws doc +comment-list` | `legacy_only` | `legacy` | no |
| `dws doc +comment-reply` | `legacy_only` | `legacy` | no |
| `dws doc +comment-update` | `legacy_only` | `legacy` | no |
| `dws doc +copy` | `legacy_only` | `legacy` | no |
| `dws doc +create` | `legacy_only` | `legacy` | no |
| `dws doc +create-from-template` | `legacy_only` | `legacy` | no |
| `dws doc +doc-append` | `legacy_only` | `legacy` | no |
| `dws doc +export` | `legacy_only` | `legacy` | no |
| `dws doc +export-get` | `legacy_only` | `legacy` | no |
| `dws doc +export-submit` | `legacy_only` | `legacy` | no |
| `dws doc +fetch` | `legacy_only` | `legacy` | no |
| `dws doc +find-doc` | `legacy_only` | `legacy` | no |
| `dws doc +grant-and-share` | `legacy_only` | `legacy` | no |
| `dws doc +history-list` | `legacy_only` | `legacy` | no |
| `dws doc +history-revert` | `dual_validate` | `legacy` | no |
| `dws doc +history-save` | `legacy_only` | `legacy` | no |
| `dws doc +import` | `legacy_only` | `legacy` | no |
| `dws doc +inspect` | `legacy_only` | `legacy` | no |
| `dws doc +list` | `unified_active` | `unified` | no |
| `dws doc +media-download` | `legacy_only` | `legacy` | no |
| `dws doc +media-insert` | `legacy_only` | `legacy` | no |
| `dws doc +media-list` | `legacy_only` | `legacy` | no |
| `dws doc +media-preview` | `legacy_only` | `legacy` | no |
| `dws doc +move` | `legacy_only` | `legacy` | no |
| `dws doc +resource-delete` | `legacy_only` | `legacy` | no |
| `dws doc +resource-download` | `legacy_only` | `legacy` | no |
| `dws doc +resource-update` | `legacy_only` | `legacy` | no |
| `dws doc +review` | `legacy_only` | `legacy` | no |
| `dws doc +search` | `unified_active` | `unified` | no |
| `dws doc +share` | `legacy_only` | `legacy` | no |
| `dws doc +share-doc` | `legacy_only` | `legacy` | no |
| `dws doc +template-apply` | `legacy_only` | `legacy` | yes |
| `dws doc +template-list` | `legacy_only` | `legacy` | no |
| `dws doc +template-search` | `legacy_only` | `legacy` | no |
| `dws doc +update` | `legacy_only` | `legacy` | no |
| `dws doc +version-list` | `legacy_only` | `legacy` | no |
| `dws doc +version-revert` | `legacy_only` | `legacy` | no |
| `dws doc +version-save` | `legacy_only` | `legacy` | no |
| `dws doc block` | `legacy_only` | `legacy` | no |
| `dws doc block delete` | `legacy_only` | `legacy` | no |
| `dws doc block insert` | `legacy_only` | `legacy` | no |
| `dws doc block list` | `legacy_only` | `legacy` | no |
| `dws doc block update` | `legacy_only` | `legacy` | no |
| `dws doc comment` | `legacy_only` | `legacy` | no |
| `dws doc comment create` | `legacy_only` | `legacy` | no |
| `dws doc comment create-inline` | `legacy_only` | `legacy` | no |
| `dws doc comment delete` | `legacy_only` | `legacy` | no |
| `dws doc comment list` | `legacy_only` | `legacy` | no |
| `dws doc comment reply` | `legacy_only` | `legacy` | no |
| `dws doc comment update` | `legacy_only` | `legacy` | no |
| `dws doc copy` | `legacy_only` | `legacy` | yes |
| `dws doc create` | `legacy_only` | `legacy` | no |
| `dws doc delete` | `legacy_only` | `legacy` | yes |
| `dws doc download` | `legacy_only` | `legacy` | yes |
| `dws doc export` | `legacy_only` | `legacy` | no |
| `dws doc export get` | `legacy_only` | `legacy` | no |
| `dws doc file` | `legacy_only` | `legacy` | no |
| `dws doc file create` | `legacy_only` | `legacy` | no |
| `dws doc file search` | `legacy_only` | `legacy` | yes |
| `dws doc folder` | `legacy_only` | `legacy` | yes |
| `dws doc folder create` | `legacy_only` | `legacy` | no |
| `dws doc import` | `legacy_only` | `legacy` | no |
| `dws doc import get` | `legacy_only` | `legacy` | no |
| `dws doc info` | `legacy_only` | `legacy` | no |
| `dws doc list` | `legacy_only` | `legacy` | yes |
| `dws doc media` | `legacy_only` | `legacy` | no |
| `dws doc media download` | `legacy_only` | `legacy` | no |
| `dws doc media insert` | `legacy_only` | `legacy` | no |
| `dws doc media upload` | `legacy_only` | `legacy` | no |
| `dws doc move` | `legacy_only` | `legacy` | yes |
| `dws doc permission` | `legacy_only` | `legacy` | yes |
| `dws doc permission add` | `legacy_only` | `legacy` | no |
| `dws doc permission list` | `legacy_only` | `legacy` | no |
| `dws doc permission remove` | `legacy_only` | `legacy` | no |
| `dws doc permission update` | `legacy_only` | `legacy` | no |
| `dws doc read` | `legacy_only` | `legacy` | no |
| `dws doc rename` | `legacy_only` | `legacy` | yes |
| `dws doc search` | `legacy_only` | `legacy` | yes |
| `dws doc style` | `legacy_only` | `legacy` | no |
| `dws doc style background` | `legacy_only` | `legacy` | no |
| `dws doc style background clear` | `legacy_only` | `legacy` | no |
| `dws doc style background set` | `legacy_only` | `legacy` | no |
| `dws doc style cover` | `legacy_only` | `legacy` | no |
| `dws doc style cover clear` | `legacy_only` | `legacy` | no |
| `dws doc style cover set` | `legacy_only` | `legacy` | no |
| `dws doc style get` | `legacy_only` | `legacy` | no |
| `dws doc template` | `legacy_only` | `legacy` | no |
| `dws doc template apply` | `legacy_only` | `legacy` | no |
| `dws doc template list` | `legacy_only` | `legacy` | no |
| `dws doc template search` | `legacy_only` | `legacy` | no |
| `dws doc update` | `legacy_only` | `legacy` | no |
| `dws doc upload` | `legacy_only` | `legacy` | yes |
| `dws doc version` | `legacy_only` | `legacy` | no |
| `dws doc version list` | `legacy_only` | `legacy` | no |
| `dws doc version revert` | `legacy_only` | `legacy` | no |
| `dws doc version save` | `legacy_only` | `legacy` | no |
| `dws doc whiteboard` | `legacy_only` | `legacy` | no |
| `dws doc whiteboard insert` | `legacy_only` | `legacy` | no |
| `dws doctor` | `legacy_only` | `legacy` | no |
| `dws drive` | `legacy_only` | `legacy` | no |
| `dws drive +copy` | `legacy_only` | `legacy` | no |
| `dws drive +download` | `legacy_only` | `legacy` | yes |
| `dws drive +find-file` | `unified_active` | `unified` | no |
| `dws drive +info` | `legacy_only` | `legacy` | no |
| `dws drive +list` | `legacy_only` | `legacy` | yes |
| `dws drive +move` | `legacy_only` | `legacy` | no |
| `dws drive +recent` | `unified_active` | `unified` | no |
| `dws drive +search` | `unified_active` | `unified` | no |
| `dws drive +search-docs` | `unified_active` | `unified` | no |
| `dws drive commit` | `legacy_only` | `legacy` | no |
| `dws drive copy` | `legacy_only` | `legacy` | no |
| `dws drive cover` | `legacy_only` | `legacy` | no |
| `dws drive delete` | `legacy_only` | `legacy` | no |
| `dws drive download` | `unified_active` | `unified` | no |
| `dws drive download-version` | `unified_active` | `unified` | no |
| `dws drive folder` | `legacy_only` | `legacy` | yes |
| `dws drive folder create` | `legacy_only` | `legacy` | no |
| `dws drive info` | `legacy_only` | `legacy` | no |
| `dws drive list` | `legacy_only` | `legacy` | no |
| `dws drive list-spaces` | `legacy_only` | `legacy` | no |
| `dws drive mkdir` | `legacy_only` | `legacy` | no |
| `dws drive move` | `legacy_only` | `legacy` | no |
| `dws drive permission` | `legacy_only` | `legacy` | no |
| `dws drive permission add` | `legacy_only` | `legacy` | no |
| `dws drive permission apply` | `legacy_only` | `legacy` | no |
| `dws drive permission apply-info` | `legacy_only` | `legacy` | no |
| `dws drive permission list` | `legacy_only` | `legacy` | no |
| `dws drive permission remove` | `legacy_only` | `legacy` | no |
| `dws drive permission transfer-owner` | `legacy_only` | `legacy` | no |
| `dws drive permission update` | `legacy_only` | `legacy` | no |
| `dws drive publish` | `legacy_only` | `legacy` | no |
| `dws drive publish get` | `legacy_only` | `legacy` | no |
| `dws drive publish set` | `legacy_only` | `legacy` | no |
| `dws drive publish unset` | `legacy_only` | `legacy` | no |
| `dws drive recent` | `legacy_only` | `legacy` | no |
| `dws drive recycle` | `legacy_only` | `legacy` | no |
| `dws drive recycle list` | `legacy_only` | `legacy` | no |
| `dws drive recycle restore` | `legacy_only` | `legacy` | no |
| `dws drive rename` | `legacy_only` | `legacy` | no |
| `dws drive revert` | `legacy_only` | `legacy` | no |
| `dws drive search` | `legacy_only` | `legacy` | no |
| `dws drive shortcut` | `legacy_only` | `legacy` | no |
| `dws drive star` | `legacy_only` | `legacy` | no |
| `dws drive star add` | `legacy_only` | `legacy` | no |
| `dws drive star list` | `legacy_only` | `legacy` | no |
| `dws drive star remove` | `legacy_only` | `legacy` | no |
| `dws drive stats` | `legacy_only` | `legacy` | no |
| `dws drive upload` | `legacy_only` | `legacy` | no |
| `dws drive upload-info` | `legacy_only` | `legacy` | no |
| `dws event` | `legacy_only` | `legacy` | no |
| `dws event +listen-im` | `legacy_only` | `legacy` | no |
| `dws event _bus` | `legacy_only` | `legacy` | yes |
| `dws event consume` | `legacy_only` | `legacy` | no |
| `dws event list` | `legacy_only` | `legacy` | no |
| `dws event schema` | `legacy_only` | `legacy` | no |
| `dws event status` | `legacy_only` | `legacy` | no |
| `dws event stop` | `unified_active` | `unified` | no |
| `dws hrbrain` | `legacy_only` | `legacy` | no |
| `dws hrbrain profile` | `legacy_only` | `legacy` | no |
| `dws hrbrain profile career` | `legacy_only` | `legacy` | no |
| `dws hrbrain profile labels` | `legacy_only` | `legacy` | no |
| `dws hrbrain profile metadata` | `legacy_only` | `legacy` | no |
| `dws hrbrain profile performance` | `legacy_only` | `legacy` | no |
| `dws hrbrain profile query` | `legacy_only` | `legacy` | no |
| `dws hrbrain search` | `legacy_only` | `legacy` | no |
| `dws hrbrain search employees` | `legacy_only` | `legacy` | no |
| `dws hrbrain search employees-structured` | `legacy_only` | `legacy` | no |
| `dws hrbrain search fields` | `legacy_only` | `legacy` | no |
| `dws hrbrain talent-pool` | `legacy_only` | `legacy` | no |
| `dws hrbrain talent-pool detail` | `legacy_only` | `legacy` | no |
| `dws hrbrain talent-pool employees` | `legacy_only` | `legacy` | no |
| `dws hrbrain talent-pool list` | `legacy_only` | `legacy` | no |
| `dws live` | `legacy_only` | `legacy` | no |
| `dws live stream` | `legacy_only` | `legacy` | no |
| `dws live stream list` | `legacy_only` | `legacy` | no |
| `dws mail` | `legacy_only` | `legacy` | no |
| `dws mail +contact-list` | `unified_active` | `unified` | no |
| `dws mail +find-mail-user` | `legacy_only` | `legacy` | no |
| `dws mail +folder-list` | `unified_active` | `unified` | no |
| `dws mail +recent-mail` | `legacy_only` | `legacy` | no |
| `dws mail +search-mail` | `legacy_only` | `legacy` | no |
| `dws mail +tag-list` | `unified_active` | `unified` | no |
| `dws mail +template-list` | `unified_active` | `unified` | no |
| `dws mail +thread-list` | `unified_active` | `unified` | no |
| `dws mail +unread-mail` | `legacy_only` | `legacy` | no |
| `dws mail +user-search` | `unified_active` | `unified` | no |
| `dws mail allow-list` | `legacy_only` | `legacy` | no |
| `dws mail allow-list add` | `legacy_only` | `legacy` | no |
| `dws mail allow-list list` | `legacy_only` | `legacy` | no |
| `dws mail allow-list remove` | `legacy_only` | `legacy` | no |
| `dws mail attachment` | `legacy_only` | `legacy` | no |
| `dws mail attachment download` | `legacy_only` | `legacy` | no |
| `dws mail attachment list` | `legacy_only` | `legacy` | no |
| `dws mail auto-reply` | `legacy_only` | `legacy` | no |
| `dws mail auto-reply get` | `legacy_only` | `legacy` | no |
| `dws mail auto-reply update` | `legacy_only` | `legacy` | no |
| `dws mail block-list` | `legacy_only` | `legacy` | no |
| `dws mail block-list add` | `legacy_only` | `legacy` | no |
| `dws mail block-list list` | `legacy_only` | `legacy` | no |
| `dws mail block-list remove` | `legacy_only` | `legacy` | no |
| `dws mail calendar` | `legacy_only` | `legacy` | no |
| `dws mail calendar list` | `legacy_only` | `legacy` | no |
| `dws mail calendar-event` | `legacy_only` | `legacy` | no |
| `dws mail calendar-event list` | `legacy_only` | `legacy` | no |
| `dws mail contact` | `legacy_only` | `legacy` | no |
| `dws mail contact batch-delete` | `legacy_only` | `legacy` | no |
| `dws mail contact create` | `legacy_only` | `legacy` | no |
| `dws mail contact list` | `legacy_only` | `legacy` | no |
| `dws mail contact update` | `legacy_only` | `legacy` | no |
| `dws mail draft` | `legacy_only` | `legacy` | no |
| `dws mail draft create` | `legacy_only` | `legacy` | no |
| `dws mail draft send` | `legacy_only` | `legacy` | no |
| `dws mail draft update` | `legacy_only` | `legacy` | no |
| `dws mail folder` | `legacy_only` | `legacy` | no |
| `dws mail folder create` | `legacy_only` | `legacy` | no |
| `dws mail folder delete` | `legacy_only` | `legacy` | no |
| `dws mail folder list` | `legacy_only` | `legacy` | no |
| `dws mail folder update` | `legacy_only` | `legacy` | no |
| `dws mail mailbox` | `legacy_only` | `legacy` | no |
| `dws mail mailbox list` | `legacy_only` | `legacy` | no |
| `dws mail mailbox profile` | `legacy_only` | `legacy` | no |
| `dws mail mailbox shared-with-me` | `legacy_only` | `legacy` | no |
| `dws mail message` | `legacy_only` | `legacy` | no |
| `dws mail message batch-delete` | `legacy_only` | `legacy` | no |
| `dws mail message batch-get` | `legacy_only` | `legacy` | no |
| `dws mail message batch-move` | `legacy_only` | `legacy` | no |
| `dws mail message batch-update` | `legacy_only` | `legacy` | no |
| `dws mail message export` | `legacy_only` | `legacy` | no |
| `dws mail message forward` | `legacy_only` | `legacy` | no |
| `dws mail message get` | `legacy_only` | `legacy` | no |
| `dws mail message list` | `legacy_only` | `legacy` | no |
| `dws mail message reply` | `legacy_only` | `legacy` | no |
| `dws mail message reply-all` | `legacy_only` | `legacy` | no |
| `dws mail message search` | `legacy_only` | `legacy` | no |
| `dws mail message send` | `legacy_only` | `legacy` | no |
| `dws mail message share-to-chat` | `legacy_only` | `legacy` | no |
| `dws mail message verify` | `legacy_only` | `legacy` | no |
| `dws mail rule` | `legacy_only` | `legacy` | no |
| `dws mail rule adjust` | `legacy_only` | `legacy` | no |
| `dws mail rule create` | `legacy_only` | `legacy` | no |
| `dws mail rule delete` | `legacy_only` | `legacy` | no |
| `dws mail rule list` | `legacy_only` | `legacy` | no |
| `dws mail rule update` | `legacy_only` | `legacy` | no |
| `dws mail sent-message` | `legacy_only` | `legacy` | no |
| `dws mail sent-message recall` | `legacy_only` | `legacy` | no |
| `dws mail sent-message recall-detail` | `legacy_only` | `legacy` | no |
| `dws mail tag` | `legacy_only` | `legacy` | no |
| `dws mail tag create` | `legacy_only` | `legacy` | no |
| `dws mail tag delete` | `legacy_only` | `legacy` | no |
| `dws mail tag list` | `legacy_only` | `legacy` | no |
| `dws mail tag update` | `legacy_only` | `legacy` | no |
| `dws mail template` | `legacy_only` | `legacy` | no |
| `dws mail template create` | `legacy_only` | `legacy` | no |
| `dws mail template delete` | `legacy_only` | `legacy` | no |
| `dws mail template get` | `legacy_only` | `legacy` | no |
| `dws mail template list` | `legacy_only` | `legacy` | no |
| `dws mail template update` | `legacy_only` | `legacy` | no |
| `dws mail thread` | `legacy_only` | `legacy` | no |
| `dws mail thread batch-trash` | `legacy_only` | `legacy` | no |
| `dws mail thread batch-update` | `legacy_only` | `legacy` | no |
| `dws mail thread get` | `legacy_only` | `legacy` | no |
| `dws mail thread list` | `legacy_only` | `legacy` | no |
| `dws mail thread trash` | `legacy_only` | `legacy` | no |
| `dws mail thread update` | `legacy_only` | `legacy` | no |
| `dws mail user` | `legacy_only` | `legacy` | no |
| `dws mail user search` | `legacy_only` | `legacy` | no |
| `dws markdown` | `legacy_only` | `legacy` | no |
| `dws markdown create` | `legacy_only` | `legacy` | no |
| `dws markdown diff` | `legacy_only` | `legacy` | no |
| `dws markdown fetch` | `legacy_only` | `legacy` | no |
| `dws markdown overwrite` | `legacy_only` | `legacy` | no |
| `dws markdown patch` | `legacy_only` | `legacy` | no |
| `dws mcp` | `legacy_only` | `legacy` | no |
| `dws mcp url` | `legacy_only` | `legacy` | no |
| `dws mcp url get` | `legacy_only` | `legacy` | no |
| `dws minutes` | `legacy_only` | `legacy` | no |
| `dws minutes +action-items` | `legacy_only` | `legacy` | yes |
| `dws minutes +detail` | `unified_active` | `unified` | no |
| `dws minutes +latest-minutes` | `legacy_only` | `legacy` | yes |
| `dws minutes +list-all` | `unified_active` | `unified` | no |
| `dws minutes +list-mine` | `unified_active` | `unified` | no |
| `dws minutes +list-shared` | `unified_active` | `unified` | no |
| `dws minutes +minutes-search` | `unified_active` | `unified` | no |
| `dws minutes +record-pause` | `legacy_only` | `legacy` | yes |
| `dws minutes +record-resume` | `legacy_only` | `legacy` | yes |
| `dws minutes +record-start` | `legacy_only` | `legacy` | no |
| `dws minutes +record-stop` | `legacy_only` | `legacy` | yes |
| `dws minutes +replace-batch` | `legacy_only` | `legacy` | no |
| `dws minutes +transcript` | `legacy_only` | `legacy` | yes |
| `dws minutes audio-memo` | `legacy_only` | `legacy` | no |
| `dws minutes audio-memo list` | `legacy_only` | `legacy` | no |
| `dws minutes get` | `legacy_only` | `legacy` | no |
| `dws minutes get audio` | `legacy_only` | `legacy` | no |
| `dws minutes get batch` | `legacy_only` | `legacy` | no |
| `dws minutes get info` | `legacy_only` | `legacy` | no |
| `dws minutes get keywords` | `legacy_only` | `legacy` | no |
| `dws minutes get summary` | `legacy_only` | `legacy` | no |
| `dws minutes get todos` | `legacy_only` | `legacy` | no |
| `dws minutes get transcription` | `legacy_only` | `legacy` | no |
| `dws minutes hot-word` | `legacy_only` | `legacy` | no |
| `dws minutes hot-word add` | `legacy_only` | `legacy` | no |
| `dws minutes hot-word delete` | `legacy_only` | `legacy` | no |
| `dws minutes hot-word list` | `legacy_only` | `legacy` | no |
| `dws minutes list` | `legacy_only` | `legacy` | no |
| `dws minutes list all` | `legacy_only` | `legacy` | no |
| `dws minutes list mine` | `legacy_only` | `legacy` | no |
| `dws minutes list shared` | `legacy_only` | `legacy` | no |
| `dws minutes mind-graph` | `legacy_only` | `legacy` | no |
| `dws minutes mind-graph create` | `legacy_only` | `legacy` | no |
| `dws minutes mind-graph status` | `legacy_only` | `legacy` | no |
| `dws minutes permission` | `legacy_only` | `legacy` | no |
| `dws minutes permission add` | `legacy_only` | `legacy` | no |
| `dws minutes permission apply` | `legacy_only` | `legacy` | no |
| `dws minutes permission remove` | `legacy_only` | `legacy` | no |
| `dws minutes record` | `legacy_only` | `legacy` | no |
| `dws minutes record pause` | `legacy_only` | `legacy` | no |
| `dws minutes record resume` | `legacy_only` | `legacy` | no |
| `dws minutes record start` | `legacy_only` | `legacy` | no |
| `dws minutes record stop` | `legacy_only` | `legacy` | no |
| `dws minutes replace-text` | `legacy_only` | `legacy` | no |
| `dws minutes speaker` | `legacy_only` | `legacy` | no |
| `dws minutes speaker replace` | `legacy_only` | `legacy` | no |
| `dws minutes speaker summary` | `legacy_only` | `legacy` | no |
| `dws minutes speaker summary create` | `legacy_only` | `legacy` | no |
| `dws minutes speaker summary get` | `legacy_only` | `legacy` | no |
| `dws minutes tag` | `legacy_only` | `legacy` | no |
| `dws minutes tag list` | `legacy_only` | `legacy` | no |
| `dws minutes tag query` | `legacy_only` | `legacy` | no |
| `dws minutes update` | `legacy_only` | `legacy` | no |
| `dws minutes update summary` | `legacy_only` | `legacy` | no |
| `dws minutes update title` | `legacy_only` | `legacy` | no |
| `dws minutes upload` | `legacy_only` | `legacy` | no |
| `dws minutes upload cancel` | `legacy_only` | `legacy` | no |
| `dws minutes upload complete` | `legacy_only` | `legacy` | no |
| `dws minutes upload create` | `legacy_only` | `legacy` | no |
| `dws oa` | `legacy_only` | `legacy` | no |
| `dws oa +approve-by` | `legacy_only` | `legacy` | yes |
| `dws oa +done-approvals` | `legacy_only` | `legacy` | no |
| `dws oa +list-cc` | `unified_active` | `unified` | no |
| `dws oa +list-executed` | `unified_active` | `unified` | no |
| `dws oa +list-forms` | `unified_active` | `unified` | no |
| `dws oa +list-pending` | `unified_active` | `unified` | no |
| `dws oa +list-submitted` | `unified_active` | `unified` | no |
| `dws oa +my-initiated` | `legacy_only` | `legacy` | no |
| `dws oa +pending` | `legacy_only` | `legacy` | no |
| `dws oa +search-forms` | `unified_active` | `unified` | no |
| `dws oa approval` | `legacy_only` | `legacy` | no |
| `dws oa approval append-task` | `legacy_only` | `legacy` | no |
| `dws oa approval approve` | `legacy_only` | `legacy` | no |
| `dws oa approval create-instance` | `legacy_only` | `legacy` | no |
| `dws oa approval detail` | `legacy_only` | `legacy` | no |
| `dws oa approval ding-info` | `legacy_only` | `legacy` | no |
| `dws oa approval forecast-process` | `legacy_only` | `legacy` | no |
| `dws oa approval form-schema` | `legacy_only` | `legacy` | no |
| `dws oa approval list-cc` | `legacy_only` | `legacy` | no |
| `dws oa approval list-executed` | `legacy_only` | `legacy` | no |
| `dws oa approval list-forms` | `legacy_only` | `legacy` | no |
| `dws oa approval list-initiated` | `legacy_only` | `legacy` | no |
| `dws oa approval list-pending` | `legacy_only` | `legacy` | no |
| `dws oa approval list-submitted` | `legacy_only` | `legacy` | no |
| `dws oa approval oa-cc-noticer` | `legacy_only` | `legacy` | no |
| `dws oa approval oa-comments` | `legacy_only` | `legacy` | no |
| `dws oa approval records` | `legacy_only` | `legacy` | no |
| `dws oa approval redirect-task` | `legacy_only` | `legacy` | no |
| `dws oa approval reject` | `legacy_only` | `legacy` | no |
| `dws oa approval revert-activities` | `legacy_only` | `legacy` | no |
| `dws oa approval revert-task` | `legacy_only` | `legacy` | no |
| `dws oa approval revoke` | `legacy_only` | `legacy` | no |
| `dws oa approval search-forms` | `legacy_only` | `legacy` | no |
| `dws oa approval tasks` | `legacy_only` | `legacy` | no |
| `dws pat` | `legacy_only` | `legacy` | no |
| `dws pat browser-policy` | `legacy_only` | `legacy` | no |
| `dws pat chmod` | `legacy_only` | `legacy` | no |
| `dws plugin` | `legacy_only` | `legacy` | no |
| `dws plugin build` | `legacy_only` | `legacy` | no |
| `dws plugin config` | `legacy_only` | `legacy` | no |
| `dws plugin config get` | `legacy_only` | `legacy` | no |
| `dws plugin config list` | `legacy_only` | `legacy` | no |
| `dws plugin config set` | `legacy_only` | `legacy` | no |
| `dws plugin config unset` | `legacy_only` | `legacy` | no |
| `dws plugin create` | `legacy_only` | `legacy` | no |
| `dws plugin dev` | `legacy_only` | `legacy` | no |
| `dws plugin disable` | `legacy_only` | `legacy` | no |
| `dws plugin enable` | `legacy_only` | `legacy` | no |
| `dws plugin info` | `legacy_only` | `legacy` | no |
| `dws plugin install` | `legacy_only` | `legacy` | no |
| `dws plugin list` | `legacy_only` | `legacy` | no |
| `dws plugin remove` | `legacy_only` | `legacy` | no |
| `dws plugin validate` | `legacy_only` | `legacy` | no |
| `dws profile` | `legacy_only` | `legacy` | no |
| `dws profile list` | `legacy_only` | `legacy` | no |
| `dws profile switch` | `legacy_only` | `legacy` | no |
| `dws profile use` | `legacy_only` | `legacy` | no |
| `dws recovery` | `legacy_only` | `legacy` | no |
| `dws recovery execute` | `legacy_only` | `legacy` | no |
| `dws recovery finalize` | `legacy_only` | `legacy` | no |
| `dws recovery plan` | `legacy_only` | `legacy` | no |
| `dws report` | `legacy_only` | `legacy` | no |
| `dws report +inbox-list` | `unified_active` | `unified` | no |
| `dws report +outbox-list` | `unified_active` | `unified` | no |
| `dws report +report-latest` | `legacy_only` | `legacy` | yes |
| `dws report create` | `legacy_only` | `legacy` | no |
| `dws report created` | `legacy_only` | `legacy` | no |
| `dws report detail` | `legacy_only` | `legacy` | no |
| `dws report entry` | `legacy_only` | `legacy` | no |
| `dws report entry get` | `legacy_only` | `legacy` | no |
| `dws report entry stats` | `legacy_only` | `legacy` | no |
| `dws report entry submit` | `legacy_only` | `legacy` | no |
| `dws report inbox` | `legacy_only` | `legacy` | no |
| `dws report inbox list` | `legacy_only` | `legacy` | no |
| `dws report list` | `legacy_only` | `legacy` | no |
| `dws report outbox` | `legacy_only` | `legacy` | no |
| `dws report outbox list` | `legacy_only` | `legacy` | no |
| `dws report sent` | `legacy_only` | `legacy` | no |
| `dws report stats` | `legacy_only` | `legacy` | no |
| `dws report template` | `legacy_only` | `legacy` | no |
| `dws report template detail` | `legacy_only` | `legacy` | no |
| `dws report template get` | `legacy_only` | `legacy` | no |
| `dws report template list` | `legacy_only` | `legacy` | no |
| `dws schema` | `legacy_only` | `legacy` | no |
| `dws sheet` | `legacy_only` | `legacy` | no |
| `dws sheet +list-sheets` | `unified_active` | `unified` | no |
| `dws sheet +read` | `legacy_only` | `legacy` | no |
| `dws sheet add-dimension` | `legacy_only` | `legacy` | no |
| `dws sheet append` | `legacy_only` | `legacy` | no |
| `dws sheet batch-update` | `legacy_only` | `legacy` | no |
| `dws sheet chart` | `legacy_only` | `legacy` | no |
| `dws sheet chart create` | `legacy_only` | `legacy` | no |
| `dws sheet chart delete` | `legacy_only` | `legacy` | no |
| `dws sheet chart list` | `legacy_only` | `legacy` | no |
| `dws sheet chart update` | `legacy_only` | `legacy` | no |
| `dws sheet comment` | `legacy_only` | `legacy` | no |
| `dws sheet comment create` | `legacy_only` | `legacy` | no |
| `dws sheet comment delete` | `legacy_only` | `legacy` | no |
| `dws sheet comment list` | `legacy_only` | `legacy` | no |
| `dws sheet comment reply` | `legacy_only` | `legacy` | no |
| `dws sheet comment update` | `legacy_only` | `legacy` | no |
| `dws sheet cond-format` | `legacy_only` | `legacy` | no |
| `dws sheet cond-format create` | `legacy_only` | `legacy` | no |
| `dws sheet cond-format delete` | `legacy_only` | `legacy` | no |
| `dws sheet cond-format list` | `legacy_only` | `legacy` | no |
| `dws sheet cond-format update` | `legacy_only` | `legacy` | no |
| `dws sheet copy` | `legacy_only` | `legacy` | no |
| `dws sheet create` | `legacy_only` | `legacy` | no |
| `dws sheet create-float-image` | `legacy_only` | `legacy` | no |
| `dws sheet csv-get` | `legacy_only` | `legacy` | no |
| `dws sheet csv-put` | `legacy_only` | `legacy` | no |
| `dws sheet delete-dimension` | `legacy_only` | `legacy` | no |
| `dws sheet delete-dropdown` | `legacy_only` | `legacy` | no |
| `dws sheet delete-float-image` | `legacy_only` | `legacy` | no |
| `dws sheet delete-sheet` | `legacy_only` | `legacy` | no |
| `dws sheet export` | `unified_active` | `unified` | no |
| `dws sheet filter` | `legacy_only` | `legacy` | no |
| `dws sheet filter clear-criteria` | `legacy_only` | `legacy` | no |
| `dws sheet filter create` | `legacy_only` | `legacy` | no |
| `dws sheet filter delete` | `legacy_only` | `legacy` | no |
| `dws sheet filter get` | `legacy_only` | `legacy` | no |
| `dws sheet filter sort` | `legacy_only` | `legacy` | no |
| `dws sheet filter update` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view create` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view delete` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view delete-criteria` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view get-criteria` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view info` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view list` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view list-criteria` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view update` | `legacy_only` | `legacy` | no |
| `dws sheet filter-view update-criteria` | `legacy_only` | `legacy` | no |
| `dws sheet find` | `legacy_only` | `legacy` | no |
| `dws sheet formula-verify` | `legacy_only` | `legacy` | no |
| `dws sheet get-dropdown` | `legacy_only` | `legacy` | no |
| `dws sheet get-float-image` | `legacy_only` | `legacy` | no |
| `dws sheet group-dimension` | `legacy_only` | `legacy` | no |
| `dws sheet hide-gridline` | `legacy_only` | `legacy` | no |
| `dws sheet import` | `legacy_only` | `legacy` | no |
| `dws sheet import create` | `legacy_only` | `legacy` | no |
| `dws sheet import get` | `legacy_only` | `legacy` | no |
| `dws sheet info` | `legacy_only` | `legacy` | no |
| `dws sheet insert-dimension` | `legacy_only` | `legacy` | no |
| `dws sheet list` | `legacy_only` | `legacy` | no |
| `dws sheet list-float-images` | `legacy_only` | `legacy` | no |
| `dws sheet media-upload` | `legacy_only` | `legacy` | no |
| `dws sheet merge-cells` | `legacy_only` | `legacy` | no |
| `dws sheet move-dimension` | `legacy_only` | `legacy` | no |
| `dws sheet new` | `legacy_only` | `legacy` | no |
| `dws sheet pivot-table` | `legacy_only` | `legacy` | no |
| `dws sheet pivot-table create` | `legacy_only` | `legacy` | no |
| `dws sheet pivot-table delete` | `legacy_only` | `legacy` | no |
| `dws sheet pivot-table list` | `legacy_only` | `legacy` | no |
| `dws sheet pivot-table update` | `legacy_only` | `legacy` | no |
| `dws sheet range` | `legacy_only` | `legacy` | no |
| `dws sheet range batch-clear` | `legacy_only` | `legacy` | no |
| `dws sheet range batch-set-style` | `legacy_only` | `legacy` | no |
| `dws sheet range clear` | `legacy_only` | `legacy` | no |
| `dws sheet range copy-to` | `legacy_only` | `legacy` | no |
| `dws sheet range fill` | `legacy_only` | `legacy` | no |
| `dws sheet range move-to` | `legacy_only` | `legacy` | no |
| `dws sheet range read` | `legacy_only` | `legacy` | no |
| `dws sheet range set-style` | `legacy_only` | `legacy` | no |
| `dws sheet range sort` | `legacy_only` | `legacy` | no |
| `dws sheet range update` | `legacy_only` | `legacy` | no |
| `dws sheet replace` | `legacy_only` | `legacy` | no |
| `dws sheet set-dropdown` | `legacy_only` | `legacy` | no |
| `dws sheet show-gridline` | `legacy_only` | `legacy` | no |
| `dws sheet table-get` | `legacy_only` | `legacy` | no |
| `dws sheet table-put` | `legacy_only` | `legacy` | no |
| `dws sheet template` | `legacy_only` | `legacy` | no |
| `dws sheet template apply` | `legacy_only` | `legacy` | no |
| `dws sheet template list` | `legacy_only` | `legacy` | no |
| `dws sheet template search` | `legacy_only` | `legacy` | no |
| `dws sheet ungroup-dimension` | `legacy_only` | `legacy` | no |
| `dws sheet unmerge-cells` | `legacy_only` | `legacy` | no |
| `dws sheet update` | `legacy_only` | `legacy` | no |
| `dws sheet update-dimension` | `legacy_only` | `legacy` | no |
| `dws sheet update-float-image` | `legacy_only` | `legacy` | no |
| `dws sheet version` | `legacy_only` | `legacy` | no |
| `dws sheet version list` | `legacy_only` | `legacy` | no |
| `dws sheet version revert` | `legacy_only` | `legacy` | no |
| `dws sheet version save` | `legacy_only` | `legacy` | no |
| `dws sheet write-image` | `legacy_only` | `legacy` | no |
| `dws shortcut add` | `legacy_only` | `legacy` | no |
| `dws shortcut list` | `legacy_only` | `legacy` | no |
| `dws shortcut stats` | `legacy_only` | `legacy` | no |
| `dws shortcut suggest` | `legacy_only` | `legacy` | no |
| `dws skill` | `legacy_only` | `legacy` | no |
| `dws skill add` | `legacy_only` | `legacy` | yes |
| `dws skill find` | `legacy_only` | `legacy` | yes |
| `dws skill get` | `legacy_only` | `legacy` | no |
| `dws skill install` | `legacy_only` | `legacy` | no |
| `dws skill search` | `legacy_only` | `legacy` | no |
| `dws skill setup` | `unified_active` | `unified` | no |
| `dws todo` | `legacy_only` | `legacy` | no |
| `dws todo +assign` | `legacy_only` | `legacy` | no |
| `dws todo +assign-multi` | `legacy_only` | `legacy` | no |
| `dws todo +created-todos` | `legacy_only` | `legacy` | no |
| `dws todo +due-today` | `legacy_only` | `legacy` | no |
| `dws todo +get` | `legacy_only` | `legacy` | no |
| `dws todo +get-my-tasks` | `unified_active` | `unified` | no |
| `dws todo +list-attachment` | `unified_active` | `unified` | no |
| `dws todo +list-comment` | `unified_active` | `unified` | no |
| `dws todo +list-sub` | `unified_active` | `unified` | no |
| `dws todo +overdue` | `legacy_only` | `legacy` | no |
| `dws todo +related-tasks` | `legacy_only` | `legacy` | no |
| `dws todo +remind` | `legacy_only` | `legacy` | no |
| `dws todo +todo-done` | `legacy_only` | `legacy` | no |
| `dws todo comment` | `legacy_only` | `legacy` | no |
| `dws todo comment add` | `legacy_only` | `legacy` | no |
| `dws todo comment delete` | `legacy_only` | `legacy` | no |
| `dws todo comment list` | `legacy_only` | `legacy` | no |
| `dws todo create` | `legacy_only` | `legacy` | yes |
| `dws todo delete` | `legacy_only` | `legacy` | yes |
| `dws todo get` | `legacy_only` | `legacy` | yes |
| `dws todo list` | `legacy_only` | `legacy` | yes |
| `dws todo tag` | `legacy_only` | `legacy` | no |
| `dws todo tag add` | `legacy_only` | `legacy` | no |
| `dws todo tag create` | `legacy_only` | `legacy` | no |
| `dws todo tag delete` | `legacy_only` | `legacy` | no |
| `dws todo tag list` | `legacy_only` | `legacy` | no |
| `dws todo tag update` | `legacy_only` | `legacy` | no |
| `dws todo task` | `legacy_only` | `legacy` | no |
| `dws todo task add-attachment` | `legacy_only` | `legacy` | no |
| `dws todo task add-executor` | `legacy_only` | `legacy` | no |
| `dws todo task add-participant` | `legacy_only` | `legacy` | no |
| `dws todo task add-reminder` | `legacy_only` | `legacy` | no |
| `dws todo task create` | `legacy_only` | `legacy` | no |
| `dws todo task create-sub` | `legacy_only` | `legacy` | no |
| `dws todo task delete` | `legacy_only` | `legacy` | no |
| `dws todo task done` | `legacy_only` | `legacy` | no |
| `dws todo task get` | `legacy_only` | `legacy` | no |
| `dws todo task list` | `legacy_only` | `legacy` | no |
| `dws todo task list-attachment` | `legacy_only` | `legacy` | no |
| `dws todo task list-sub` | `legacy_only` | `legacy` | no |
| `dws todo task remove-attachment` | `legacy_only` | `legacy` | no |
| `dws todo task remove-executor` | `legacy_only` | `legacy` | no |
| `dws todo task remove-participant` | `legacy_only` | `legacy` | no |
| `dws todo task reset-reminder` | `legacy_only` | `legacy` | no |
| `dws todo task update` | `legacy_only` | `legacy` | no |
| `dws upgrade` | `legacy_only` | `legacy` | no |
| `dws version` | `legacy_only` | `legacy` | no |
| `dws whiteboard` | `legacy_only` | `legacy` | no |
| `dws whiteboard query` | `legacy_only` | `legacy` | no |
| `dws whiteboard update` | `legacy_only` | `legacy` | no |
| `dws wiki` | `legacy_only` | `legacy` | no |
| `dws wiki +node-copy` | `legacy_only` | `legacy` | yes |
| `dws wiki +node-list` | `unified_active` | `unified` | no |
| `dws wiki +node-move` | `legacy_only` | `legacy` | yes |
| `dws wiki +resolve-space` | `legacy_only` | `legacy` | no |
| `dws wiki +space-list` | `legacy_only` | `legacy` | no |
| `dws wiki +space-search` | `legacy_only` | `legacy` | no |
| `dws wiki +wiki-new-doc` | `legacy_only` | `legacy` | yes |
| `dws wiki create` | `legacy_only` | `legacy` | yes |
| `dws wiki delete` | `legacy_only` | `legacy` | yes |
| `dws wiki doc` | `legacy_only` | `legacy` | yes |
| `dws wiki doc list` | `legacy_only` | `legacy` | yes |
| `dws wiki doc read` | `legacy_only` | `legacy` | yes |
| `dws wiki doc search` | `legacy_only` | `legacy` | yes |
| `dws wiki feed` | `legacy_only` | `legacy` | no |
| `dws wiki feed list` | `legacy_only` | `legacy` | no |
| `dws wiki file` | `legacy_only` | `legacy` | yes |
| `dws wiki file list` | `legacy_only` | `legacy` | yes |
| `dws wiki file search` | `legacy_only` | `legacy` | yes |
| `dws wiki get` | `legacy_only` | `legacy` | yes |
| `dws wiki list` | `legacy_only` | `legacy` | yes |
| `dws wiki member` | `legacy_only` | `legacy` | no |
| `dws wiki member add` | `legacy_only` | `legacy` | no |
| `dws wiki member list` | `legacy_only` | `legacy` | no |
| `dws wiki member remove` | `legacy_only` | `legacy` | no |
| `dws wiki member update` | `legacy_only` | `legacy` | no |
| `dws wiki node` | `legacy_only` | `legacy` | no |
| `dws wiki node copy` | `legacy_only` | `legacy` | no |
| `dws wiki node create` | `legacy_only` | `legacy` | no |
| `dws wiki node delete` | `legacy_only` | `legacy` | no |
| `dws wiki node list` | `legacy_only` | `legacy` | no |
| `dws wiki node move` | `legacy_only` | `legacy` | no |
| `dws wiki node search` | `legacy_only` | `legacy` | no |
| `dws wiki search` | `legacy_only` | `legacy` | yes |
| `dws wiki space` | `legacy_only` | `legacy` | no |
| `dws wiki space create` | `legacy_only` | `legacy` | no |
| `dws wiki space delete` | `legacy_only` | `legacy` | no |
| `dws wiki space get` | `legacy_only` | `legacy` | no |
| `dws wiki space list` | `legacy_only` | `legacy` | no |
| `dws wiki space search` | `legacy_only` | `legacy` | no |

## Review boundary

- 本 inventory 只证明当前 command declaration 与 rollout state；不证明业务请求、服务端终态、分页覆盖或真实账号安全性。
- `dual_validate` 仍应保持 legacy stdout；所有晋级必须由 Agent 另行核对 legacy golden、统一结果样本、非零退出码与回滚计划。
- `REVIEW` 表示跳级或未记录回退；它不是自动阻断。人类发布负责人必须决定是否附上批准与理由。
