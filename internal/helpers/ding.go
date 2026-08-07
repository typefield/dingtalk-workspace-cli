package helpers

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

// ──────────────────────────────────────────────────────────
// dws ding — DING 消息
// ──────────────────────────────────────────────────────────

// remindType: 服务端 API 1=应用内 2=短信 3=电话
var dingRemindTypeMap = map[string]int{"app": 1, "sms": 2, "call": 3}

var dingListTypes = map[string]struct{}{
	"ALL": {}, "UNREAD": {}, "SEND": {}, "NEW_COMMENT": {}, "DELETED": {},
}

func reviewedUnpinnedDingInterface(rpcName string) *contract.InterfaceSpec {
	return &contract.InterfaceSpec{
		Mode:         "composite",
		Availability: "available",
		Reason:       fmt.Sprintf("Reviewed unpinned remote adapter: the executable CLI calls im/%s, which is absent from the pinned MCP metadata snapshot; no pinned semantically equivalent interface_ref can represent the command.", rpcName),
	}
}

func validatedDingRemindType(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := dingRemindTypeMap[value]; !ok {
		return "", apperrors.NewValidation(
			fmt.Sprintf("--type must be one of app, sms, or call, got %q", raw),
			apperrors.WithReason("invalid_argument"),
		)
	}
	return value, nil
}

func validatedDingRecipients(raw string) ([]string, error) {
	values := parseCSVValues(raw)
	if len(values) == 0 {
		return nil, apperrors.NewValidation(
			"--users must contain at least one non-empty recipient ID",
			apperrors.WithReason("invalid_argument"),
		)
	}
	return values, nil
}

func newDingCommand() *cobra.Command {
	// Product-level Agent routing Decl (migrated from selection/ding.json
	// products.ding). Catalog assembly stamps provenance contract_final.
	contract.RegisterProductDecl(contract.ProductDecl{
		ID: "ding",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "查询、发送或撤回 DING，支持本人身份和企业机器人身份",
			UseWhen: []string{
				"需要查询 DING 历史或接收人已读状态",
				"需要以本人或企业机器人身份发送、撤回应用内/短信/电话 DING",
			},
			AvoidWhen: []string{
				"普通聊天消息使用 chat；发送前必须区分本人身份与机器人身份",
				"短信和电话 DING 有成本，目标、内容和提醒类型未确认时不要执行",
			},
		},
	})
	root := &cobra.Command{
		Use:   "ding",
		Short: "DING 消息 / 发送 / 撤回",
		Long:  `发送和撤回 DING 消息（应用内/短信/电话）。预发环境可用。`,
		RunE:  groupRunE,
	}

	dingMessageCmd := &cobra.Command{Use: "message", Short: "DING 消息管理", RunE: groupRunE}

	dingMessageSendCmd := &cobra.Command{
		Use:   "send",
		Short: "发送 DING 消息",
		Long: `发送 DING 消息。类型:
  app  = 应用内 DING (默认)
  sms  = 短信 DING (有成本)
  call = 电话 DING (有成本)`,
		Example: `  # 查询 userId: dws contact user search --keyword "姓名"
  dws ding message send --robot-code <robot-code> --users userId1,userId2 --content "请查看"
  dws ding message send --robot-code <robot-code> --type call --users userId1 --content "紧急告警"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlags(cmd, "users", "content"); err != nil {
				return err
			}
			robotCode := mustGetFlag(cmd, "robot-code")
			if robotCode == "" {
				robotCode = os.Getenv("DINGTALK_DING_ROBOT_CODE")
			}
			if robotCode == "" {
				return apperrors.NewValidation("flag --robot-code is required (or set DINGTALK_DING_ROBOT_CODE env var)", apperrors.WithReason("missing_required_flags"))
			}
			typeStr, err := validatedDingRemindType(mustGetFlag(cmd, "type"))
			if err != nil {
				return err
			}
			receiverUserIdList, err := validatedDingRecipients(mustGetFlag(cmd, "users"))
			if err != nil {
				return err
			}
			return callMCPTool("send_ding_message", map[string]any{
				"robotCode":          robotCode,
				"remindType":         dingRemindTypeMap[typeStr],
				"receiverUserIdList": receiverUserIdList,
				"content":            mustGetFlag(cmd, "content"),
			})
		},
	}
	DeclareLeafMetadata(dingMessageSendCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "user_required", Idempotency: "unknown",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "ding",
				Name:           "send_ding_message",
				CanonicalPath:  "ding.send_ding_message",
				CLIPath:        "ding message send",
				PrimaryCLIPath: "ding message send",
			},
			Description: "以企业机器人发送应用内/短信/电话 DING",
			Interface: &contract.InterfaceSpec{
				Mode:         "mcp",
				Availability: "available",
				Ref:          &contract.InterfaceRefSpec{ProductID: "ding", RPCName: "send_ding_message"},
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "以企业机器人发送应用内/短信/电话 DING",
				UseWhen:      []string{"需要用企业机器人向指定 userId 发送应用内、短信或电话 DING"},
				AvoidWhen: []string{
					"普通聊天消息用 chat message send / send-by-bot",
					"需要用户身份 DING 时不要用本命令（机器人身份）",
					"短信/电话有成本，用户未确认前不要发 call/sms",
				},
				Examples: []string{
					"dws ding message send --robot-code <ROBOT_CODE> --users userId1,userId2 --content \"请查看\" --format json",
					"dws ding message send --robot-code <ROBOT_CODE> --type call --users userId1 --content \"紧急告警\" --format json",
				},
			},
			Parameters: []contract.ParamDecl{
				{Name: "content", Required: boolPtr(true)},
				{Name: "robot-code", Required: boolPtr(false)},
				{Name: "type", Property: "remindType"},
				{Name: "users", Property: "receiverUserIdList", Required: boolPtr(true), InterfaceType: "array"},
			},
			DryRun: &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewRequest, RemoteReads: false},
		},
	})

	dingMessageRecallCmd := &cobra.Command{
		Use:     "recall",
		Short:   "撤回 DING 消息",
		Example: `  dws ding message recall --robot-code <robot-code> --id <open-ding-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlags(cmd, "id"); err != nil {
				return err
			}
			robotCode := mustGetFlag(cmd, "robot-code")
			if robotCode == "" {
				robotCode = os.Getenv("DINGTALK_DING_ROBOT_CODE")
			}
			if robotCode == "" {
				return apperrors.NewValidation("flag --robot-code is required (or set DINGTALK_DING_ROBOT_CODE env var)", apperrors.WithReason("missing_required_flags"))
			}
			return callMCPTool("recall_ding_message", map[string]any{
				"robotCode":  robotCode,
				"openDingId": mustGetFlag(cmd, "id"),
			})
		},
	}
	DeclareLeafMetadata(dingMessageRecallCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "destructive", Risk: "high",
			Confirmation: "user_required", Idempotency: "unknown",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "ding",
				Name:           "recall_ding_message",
				CanonicalPath:  "ding.recall_ding_message",
				CLIPath:        "ding message recall",
				PrimaryCLIPath: "ding message recall",
			},
			Description: "撤回已发送的机器人 DING",
			Interface: &contract.InterfaceSpec{
				Mode:         "mcp",
				Availability: "available",
				Ref:          &contract.InterfaceRefSpec{ProductID: "ding", RPCName: "recall_ding_message"},
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "撤回已发送的机器人 DING",
				UseWhen:      []string{"已知 openDingId 与同一 robot-code，需要撤回机器人 DING"},
				AvoidWhen:    []string{"需要以用户身份撤回 DING 时不要使用本命令"},
				Examples:     []string{"dws ding message recall --robot-code <ROBOT_CODE> --id <OPEN_DING_ID> --format json"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "id", Property: "openDingId", Required: boolPtr(true)},
				{Name: "robot-code", Property: "robotCode", Required: boolPtr(false)},
			},
			DryRun: &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewRequest, RemoteReads: false},
		},
	})

	dingMessageListCmd := &cobra.Command{
		Use:   "list",
		Short: "查询 DING 消息历史",
		Long: `查询当前用户的 DING 消息列表，支持按类型过滤。
--type 支持: ALL(全部)、UNREAD(未读)、SEND(已发)、NEW_COMMENT(新评论)、DELETED(已删除)。
--type 为服务端必填字段，空值会报「type不能为空」；不传时 CLI 默认按 ALL 查询。
列表项会返回 DING 内容，调用方可在同一结果中读取 openDingId、状态与 content，无需再发起详情查询。`,
		Example: `  dws ding message list                 # 默认 --type ALL
  dws ding message list --type UNREAD
  dws ding message list --type SEND --cursor 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			toolArgs := map[string]any{}
			cursor, _ := cmd.Flags().GetInt64("cursor")
			if cursor < 0 {
				return apperrors.NewValidation(fmt.Sprintf("--cursor must be zero or greater, got %d", cursor))
			}
			if cursor > 0 {
				toolArgs["cursor"] = cursor
			}
			// type 是服务端必填，空值会报错；不传或传空时兜底为 ALL。
			messageType, _ := cmd.Flags().GetString("type")
			messageType = strings.TrimSpace(messageType)
			if messageType == "" {
				messageType = "ALL"
			}
			if _, ok := dingListTypes[messageType]; !ok {
				return apperrors.NewValidation(fmt.Sprintf("--type must be one of ALL, UNREAD, SEND, NEW_COMMENT, or DELETED, got %q", messageType))
			}
			toolArgs["type"] = messageType
			return callMCPToolOnServer("im", "list_ding_messages", toolArgs)
		},
	}
	DeclareLeafMetadata(dingMessageListCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "ding",
				Name:           "list_ding_messages",
				CanonicalPath:  "ding.list_ding_messages",
				CLIPath:        "ding message list",
				PrimaryCLIPath: "ding message list",
			},
			Description: "查询当前用户的 DING 消息历史",
			Interface:   reviewedUnpinnedDingInterface("list_ding_messages"),
			Selection: contract.SelectionSpec{
				AgentSummary: "查询当前用户收到或发出的 DING 消息历史",
				UseWhen:      []string{"需要按类型查看 DING 历史、取得 openDingId，或继续 cursor 分页时"},
				AvoidWhen:    []string{"已知 openDingId 并只查询接收状态时使用 ding message receiver-status；发送或撤回 DING 不使用本命令"},
				Examples:     []string{"dws ding message list --type UNREAD --cursor 0"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "cursor", Property: "cursor", Required: boolPtr(false), InterfaceType: "integer"},
				{Name: "type", Property: "type", Required: boolPtr(false)},
			},
		},
	})

	dingMessageReceiverStatusCmd := &cobra.Command{
		Use:   "receiver-status",
		Short: "查看 DING 接收状态",
		Long:  `查看指定 DING 消息的接收者状态（已读/未读等）。`,
		Example: `  dws ding message receiver-status --ding-id <openDingId>
  # 查询 dingId: dws ding message list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlags(cmd, "ding-id"); err != nil {
				return err
			}
			return callMCPToolOnServer("im", "list_ding_receiver_status", map[string]any{
				"openDingId": mustGetFlag(cmd, "ding-id"),
			})
		},
	}
	DeclareLeafMetadata(dingMessageReceiverStatusCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "ding",
				Name:           "list_ding_receiver_status",
				CanonicalPath:  "ding.list_ding_receiver_status",
				CLIPath:        "ding message receiver-status",
				PrimaryCLIPath: "ding message receiver-status",
			},
			Description: "查询指定 DING 的接收人已读状态",
			Interface:   reviewedUnpinnedDingInterface("list_ding_receiver_status"),
			Selection: contract.SelectionSpec{
				AgentSummary: "查询指定 DING 的接收人已读或未读状态",
				UseWhen:      []string{"已有 openDingId，需要确认每位接收人是否已读以便跟进时"},
				AvoidWhen:    []string{"不知道 openDingId 时先使用 ding message list；本命令不发送催办消息"},
				Examples:     []string{"dws ding message receiver-status --ding-id <OPEN_DING_ID>"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "ding-id", Property: "openDingId", Required: boolPtr(true)},
			},
		},
	})

	// ── send-personal: 以用户身份发送 DING ──────────────────────

	dingMessageSendPersonalCmd := &cobra.Command{
		Use:   "send-personal",
		Short: "以用户身份发送 DING",
		Long: `以当前用户身份（非机器人）发送 DING 消息。提醒类型:
  app  = 应用内 DING (默认)
  sms  = 短信 DING (有成本)
  call = 电话 DING (有成本)`,
		Example: `  # 查询 openDingTalkId: dws contact user search --query "姓名"
  dws ding message send-personal --users openDingTalkId1,openDingTalkId2 --content "请查看"
  dws ding message send-personal --type call --users openDingTalkId1 --content "紧急告警"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlags(cmd, "users", "content"); err != nil {
				return err
			}
			users, err := validatedDingRecipients(mustGetFlag(cmd, "users"))
			if err != nil {
				return err
			}
			remindType, err := validatedDingRemindType(mustGetFlag(cmd, "type"))
			if err != nil {
				return err
			}
			toolArgs := map[string]any{
				"receiverOpenDingTalkIds": users,
				"content":                 mustGetFlag(cmd, "content"),
				"remindType":              remindType,
			}
			if v, _ := cmd.Flags().GetString("uuid"); v != "" {
				toolArgs["uuid"] = v
			}
			return callMCPToolOnServer("im", "send_personal_ding", toolArgs)
		},
	}
	DeclareLeafMetadata(dingMessageSendPersonalCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "user_required", Idempotency: "unknown",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "ding",
				Name:           "send_personal_ding",
				CanonicalPath:  "ding.send_personal_ding",
				CLIPath:        "ding message send-personal",
				PrimaryCLIPath: "ding message send-personal",
			},
			Description: "以当前用户身份向指定人员发送 DING",
			Interface:   reviewedUnpinnedDingInterface("send_personal_ding"),
			Selection: contract.SelectionSpec{
				AgentSummary: "以当前用户身份向指定 openDingTalkId 发送应用内、短信或电话 DING",
				UseWhen:      []string{"用户明确要求以本人身份向指定人员发送 DING，且接收人、内容和提醒类型已确认时"},
				AvoidWhen:    []string{"机器人身份发送使用 ding message send；普通聊天消息使用 chat；短信或电话有成本，未明确确认时不要执行"},
				Examples:     []string{"dws ding message send-personal --users <OPEN_DINGTALK_IDS> --content \"请查看\" --type app"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "content", Property: "content", Required: boolPtr(true)},
				{Name: "type", Property: "remindType", Required: boolPtr(false)},
				{Name: "users", Property: "receiverOpenDingTalkIds", Required: boolPtr(true), InterfaceType: "array"},
				{Name: "uuid", Property: "uuid", Required: boolPtr(false)},
			},
			DryRun: &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewRequest, RemoteReads: false},
		},
	})

	// ── send-by-message: 消息转 DING ─────────────────────────────

	dingMessageSendByMessageCmd := &cobra.Command{
		Use:   "send-by-message",
		Short: "消息转 DING（将聊天消息转为 DING 通知）",
		Long: `将指定聊天消息转为 DING 发送给指定接收者。提醒类型:
  app  = 应用内 DING (默认)
  sms  = 短信 DING (有成本)
  call = 电话 DING (有成本)`,
		Example: `  # 查询 openDingTalkId: dws contact user search --query "姓名"
  # 查询 openConversationId: dws chat search --keyword "群名"
  dws ding message send-by-message --group <openConversationId> --message-id <openMessageId> --users id1,id2
  dws ding message send-by-message --group <openConversationId> --message-id <openMessageId> --users id1 --type sms`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlags(cmd, "group", "message-id", "users"); err != nil {
				return err
			}
			users, err := validatedDingRecipients(mustGetFlag(cmd, "users"))
			if err != nil {
				return err
			}
			remindType, err := validatedDingRemindType(mustGetFlag(cmd, "type"))
			if err != nil {
				return err
			}
			toolArgs := map[string]any{
				"openConversationId":      mustGetFlag(cmd, "group"),
				"openMessageId":           mustGetFlag(cmd, "message-id"),
				"receiverOpenDingTalkIds": users,
				"remindType":              remindType,
			}
			if v, _ := cmd.Flags().GetString("uuid"); v != "" {
				toolArgs["uuid"] = v
			}
			return callMCPToolOnServer("im", "send_ding_by_message", toolArgs)
		},
	}
	DeclareLeafMetadata(dingMessageSendByMessageCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "user_required", Idempotency: "unknown",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "ding",
				Name:           "send_ding_by_message",
				CanonicalPath:  "ding.send_ding_by_message",
				CLIPath:        "ding message send-by-message",
				PrimaryCLIPath: "ding message send-by-message",
			},
			Description: "把指定聊天消息转换为面向指定接收人的 DING 提醒",
			Interface:   reviewedUnpinnedDingInterface("send_ding_by_message"),
			Selection: contract.SelectionSpec{
				AgentSummary: "把已有聊天消息转换为应用内、短信或电话 DING 提醒",
				UseWhen:      []string{"已有会话、消息和接收人 ID，用户明确要求把该消息作为 DING 强提醒时"},
				AvoidWhen:    []string{"需要自定义新内容时使用 send-personal；普通转发消息不要使用 DING；短信或电话未确认成本时不要执行"},
				Examples:     []string{"dws ding message send-by-message --group <OPEN_CONVERSATION_ID> --message-id <OPEN_MESSAGE_ID> --users <OPEN_DINGTALK_IDS> --type app"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "group", Property: "openConversationId", Required: boolPtr(true)},
				{Name: "message-id", Property: "openMessageId", Required: boolPtr(true)},
				{Name: "type", Property: "remindType", Required: boolPtr(false)},
				{Name: "users", Property: "receiverOpenDingTalkIds", Required: boolPtr(true), InterfaceType: "array"},
				{Name: "uuid", Property: "uuid", Required: boolPtr(false)},
			},
			DryRun: &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewRequest, RemoteReads: false},
		},
	})

	// ── recall-personal: 以用户身份撤回 DING ────────────────────

	dingMessageRecallPersonalCmd := &cobra.Command{
		Use:   "recall-personal",
		Short: "以用户身份撤回 DING",
		Long:  `以当前用户身份撤回已发送的 DING 消息。需要提供发送时返回的 openDingId。`,
		Example: `  dws ding message recall-personal --id <openDingId>
  # 查询 openDingId: dws ding message list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlags(cmd, "id"); err != nil {
				return err
			}
			return callMCPToolOnServer("im", "recall_personal_ding", map[string]any{
				"openDingId": mustGetFlag(cmd, "id"),
			})
		},
	}
	DeclareLeafMetadata(dingMessageRecallPersonalCmd, LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "destructive", Risk: "high",
			Confirmation: "user_required", Idempotency: "unknown",
		},
		Contract: LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "ding",
				Name:           "recall_personal_ding",
				CanonicalPath:  "ding.recall_personal_ding",
				CLIPath:        "ding message recall-personal",
				PrimaryCLIPath: "ding message recall-personal",
			},
			Description: "撤回当前用户已经发送的 DING",
			Interface:   reviewedUnpinnedDingInterface("recall_personal_ding"),
			Selection: contract.SelectionSpec{
				AgentSummary: "撤回当前用户已经发送的指定 DING",
				UseWhen:      []string{"用户明确要求撤回本人发送的 DING，且 openDingId 已核对时"},
				AvoidWhen:    []string{"机器人发送的 DING 使用 ding message recall；只查询接收状态不要撤回"},
				Examples:     []string{"dws ding message recall-personal --id <OPEN_DING_ID>"},
			},
			Parameters: []contract.ParamDecl{
				{Name: "id", Property: "openDingId", Required: boolPtr(true)},
			},
			DryRun: &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewRequest, RemoteReads: false},
		},
	})

	dingMessageSendCmd.Flags().String("robot-code", "", "机器人 ID，发 DING 的机器人编码 (必填，可从 应用管理→机器人 获取，或设 DINGTALK_DING_ROBOT_CODE)")
	dingMessageSendCmd.Flags().String("type", "app", "提醒类型: app/sms/call (默认 app)")
	dingMessageSendCmd.Flags().String("users", "", "接收人 userId 列表 (必填)")
	_ = dingMessageSendCmd.MarkFlagRequired("users")
	dingMessageSendCmd.Flags().String("content", "", "消息内容 (必填)")
	_ = dingMessageSendCmd.MarkFlagRequired("content")
	dingMessageRecallCmd.Flags().String("robot-code", "", "机器人 ID (必填，或设 DINGTALK_DING_ROBOT_CODE)")
	dingMessageRecallCmd.Flags().String("id", "", "DING 消息 ID (必填)")
	_ = dingMessageRecallCmd.MarkFlagRequired("id")
	dingMessageListCmd.Flags().Int64("cursor", 0, "分页游标（首次传 0，翻页传返回的 nextCursor）")
	dingMessageListCmd.Flags().String("type", "ALL", "消息类型: ALL / UNREAD / SEND / NEW_COMMENT / DELETED（必填，服务端不接受空值；默认 ALL 全部）")
	dingMessageReceiverStatusCmd.Flags().String("ding-id", "", "DING 消息 openDingId (必填)")
	_ = dingMessageReceiverStatusCmd.MarkFlagRequired("ding-id")
	dingMessageSendPersonalCmd.Flags().String("users", "", "接收者 openDingTalkId 列表，逗号分隔 (必填)")
	_ = dingMessageSendPersonalCmd.MarkFlagRequired("users")
	dingMessageSendPersonalCmd.Flags().String("content", "", "DING 内容 (必填)")
	_ = dingMessageSendPersonalCmd.MarkFlagRequired("content")
	dingMessageSendPersonalCmd.Flags().String("type", "app", "提醒类型: app/sms/call (默认 app)")
	dingMessageSendPersonalCmd.Flags().String("uuid", "", "幂等唯一标识（可选，不传由服务端生成）")
	dingMessageRecallPersonalCmd.Flags().String("id", "", "DING 消息 openDingId (必填)")
	_ = dingMessageRecallPersonalCmd.MarkFlagRequired("id")
	dingMessageSendByMessageCmd.Flags().String("group", "", "原消息所在会话 openConversationId (必填)")
	_ = dingMessageSendByMessageCmd.MarkFlagRequired("group")
	dingMessageSendByMessageCmd.Flags().String("message-id", "", "原消息 openMessageId (必填)")
	_ = dingMessageSendByMessageCmd.MarkFlagRequired("message-id")
	dingMessageSendByMessageCmd.Flags().String("users", "", "接收者 openDingTalkId 列表，逗号分隔 (必填)")
	_ = dingMessageSendByMessageCmd.MarkFlagRequired("users")
	dingMessageSendByMessageCmd.Flags().String("type", "app", "提醒类型: app/sms/call (默认 app)")
	dingMessageSendByMessageCmd.Flags().String("uuid", "", "幂等唯一标识（可选，不传由服务端生成）")
	dingMessageCmd.AddCommand(dingMessageSendCmd, dingMessageRecallCmd, dingMessageListCmd, dingMessageReceiverStatusCmd, dingMessageSendPersonalCmd, dingMessageRecallPersonalCmd, dingMessageSendByMessageCmd)
	root.AddCommand(dingMessageCmd)
	return root
}
