// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestChatFinalExclusionsPublishReviewedContracts(t *testing.T) {
	tests := []struct {
		name         string
		path         []string
		canonical    string
		rpc          string
		effect       string
		risk         string
		idempotency  string
		parameters   map[string]bool
		positionals  int
		statusValues []string
	}{
		{
			name: "chmod", path: []string{"chmod"}, canonical: "chat.chat_permission_grant", rpc: "chat_permission_grant",
			effect: "write", risk: "high", idempotency: "unknown", positionals: 1,
			parameters: map[string]bool{"agentCode": false, "conversation-id": false, "grant-type": false, "open-dingtalk-id": false, "permParam": false, "session-id": false, "ttl": false, "user": false},
		},
		{
			name: "cross_org", path: []string{"data-auth", "cross-org"}, canonical: "chat.chat_permission_grant_cross_org_data", rpc: "chat_permission_grant",
			effect: "write", risk: "high", idempotency: "unknown",
			parameters: map[string]bool{"agentCode": false, "all": false, "grant-type": false, "session-id": false, "target-org-id": false, "ttl": false},
		},
		{
			name: "clear_messages", path: []string{"clear-messages"}, canonical: "chat.clear_conversation_messages", rpc: "clear_conversation_messages",
			effect: "destructive", risk: "high", idempotency: "idempotent",
			parameters: map[string]bool{"conversation-id": true},
		},
		{
			name: "audit_join", path: []string{"group", "audit-join-validation"}, canonical: "chat.audit_join_group", rpc: "audit_join_group",
			effect: "write", risk: "high", idempotency: "unknown", statusValues: []string{"AuditApprove", "AuditDelete"},
			parameters: map[string]bool{"applicant": true, "description": false, "group": true, "inviter": true, "record-id": true, "status": true},
		},
	}

	root := newChatCommand()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd, remaining, err := root.Find(test.path)
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("chat %v is not an exact runnable leaf: cmd=%v remaining=%v err=%v", test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := "chat " + strings.Join(test.path, " ")
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonical || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Safety == nil || final.Safety.Effect != test.effect || final.Safety.Risk != test.risk || final.Safety.Confirmation != "user_required" || final.Safety.Idempotency != test.idempotency {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
			}
			if !HasContractConfirmSafety(cmd) {
				t.Fatalf("%s does not install the declared confirmation gate", cmd.CommandPath())
			}
			if final.Interface == nil || final.Interface.Mode != contract.InterfaceModeMCP || final.Interface.Availability != contract.InterfaceAvailable || final.Interface.Ref == nil || final.Interface.Ref.ProductID != "im" || final.Interface.Ref.RPCName != test.rpc {
				t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
			}
			if final.DryRun == nil || final.DryRun.PreviewKind != contract.DryRunPreviewRequest || final.DryRun.RemoteReads {
				t.Fatalf("%s dry_run=%#v", cmd.CommandPath(), final.DryRun)
			}
			if final.Selection == nil || strings.TrimSpace(final.Selection.AgentSummary) == "" || len(final.Selection.UseWhen) == 0 || len(final.Selection.AvoidWhen) == 0 || len(final.Selection.Examples) == 0 {
				t.Fatalf("%s selection=%#v", cmd.CommandPath(), final.Selection)
			}
			if len(final.Positionals) != test.positionals {
				t.Fatalf("%s positionals=%#v, want count=%d", cmd.CommandPath(), final.Positionals, test.positionals)
			}
			gotParameters := make(map[string]bool, len(final.Parameters))
			for _, parameter := range final.Parameters {
				gotParameters[parameter.Name] = parameter.Required != nil && *parameter.Required
				if parameter.Name == "status" && !reflect.DeepEqual(parameter.Enum, test.statusValues) {
					t.Fatalf("%s status enum=%#v, want %#v", cmd.CommandPath(), parameter.Enum, test.statusValues)
				}
			}
			if !reflect.DeepEqual(gotParameters, test.parameters) {
				t.Fatalf("%s parameters=%#v, want %#v", cmd.CommandPath(), final.Parameters, test.parameters)
			}
		})
	}
}

func TestChatFinalExclusionsRouteReviewedArgumentsExactlyOnce(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		rpc        string
		want       map[string]any
		grantParam map[string]string
	}{
		{
			name: "chmod", rpc: "chat_permission_grant",
			args:       []string{"chmod", " chat.message:send ", "--agentCode", " agent-1 ", "--grant-type", "timed", "--ttl", " 24h ", "--conversation-id", " cid-1 ", "--yes"},
			want:       map[string]any{"agentCode": "agent-1", "scope": "chat.message:send", "grantType": "timed", "ttl": "24h"},
			grantParam: map[string]string{"conversationId": "cid-1", "openConversationId": "cid-1", "openCid": "cid-1"},
		},
		{
			name: "cross_org", rpc: "chat_permission_grant",
			args:       []string{"data-auth", "cross-org", "--target-org-id", " org-1 ", "--agentCode", " agent-2 ", "--grant-type", "session", "--session-id", " session-2 ", "--yes"},
			want:       map[string]any{"agentCode": "agent-2", "scope": "chat.data:cross-org", "grantType": "session", "sessionId": "session-2", "grantCategory": "data"},
			grantParam: map[string]string{"targetOrgId": "org-1"},
		},
		{
			name: "clear_messages", rpc: "clear_conversation_messages",
			args: []string{"clear-messages", "--conversation-id", " cid-3 ", "--yes"},
			want: map[string]any{"openConversationId": "cid-3"},
		},
		{
			name: "audit_join", rpc: "audit_join_group",
			args: []string{"group", "audit-join-validation", "--group", " cid-4 ", "--record-id", " 42 ", "--applicant", " user-4 ", "--inviter", " user-5 ", "--status", "AuditApprove", "--description", "同意加入", "--yes"},
			want: map[string]any{"openConversationId": "cid-4", "applyRecordId": int64(42), "applicantUid": "user-4", "inviterUid": "user-5", "status": "AuditApprove", "auditDescription": "同意加入"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}}
			if err := runChatCoverageCommand(t, caller, test.args...); err != nil {
				t.Fatalf("execute %v: %v", test.args, err)
			}
			if caller.calls != 1 || caller.server != "im" || caller.tool != test.rpc {
				t.Fatalf("call=(count=%d server=%q tool=%q), want once im/%s", caller.calls, caller.server, caller.tool, test.rpc)
			}
			got := make(map[string]any, len(caller.args))
			for key, value := range caller.args {
				got[key] = value
			}
			if test.grantParam != nil {
				raw, ok := got["grantParams"].(string)
				if !ok {
					t.Fatalf("grantParams=%#v, want JSON string", got["grantParams"])
				}
				var decoded map[string]string
				if err := unmarshalJSONUseNumber(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, test.grantParam) {
					t.Fatalf("grantParams=%q decoded=%#v err=%v, want %#v", raw, decoded, err, test.grantParam)
				}
				delete(got, "grantParams")
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("args=%#v, want %#v", got, test.want)
			}
		})
	}
}

func TestChatFinalExclusionsRejectInvalidInputBeforeConfirmationOrRemoteCall(t *testing.T) {
	for _, args := range [][]string{
		{"chmod", "calendar.event:create", "--conversation-id", "cid-1"},
		{"chmod", "chat.message:send"},
		{"chmod", "chat.message:send", "--conversation-id", "cid-1", "--user", "user-1"},
		{"chmod", "chat.message:send", "--permParam", "broken"},
		{"chmod", "chat.message:send", "--conversation-id", "cid-1", "--grant-type", "invalid"},
		{"chmod", "chat.message:send", "--conversation-id", "cid-1", "--grant-type", "session"},
		{"chmod", "chat.message:send", "--conversation-id", "cid-1", "--ttl="},
		{"data-auth", "cross-org"},
		{"data-auth", "cross-org", "--target-org-id", "org-1", "--all"},
		{"data-auth", "cross-org", "--all", "--grant-type", "session"},
		{"clear-messages", "--conversation-id", " "},
		{"group", "audit-join-validation", "--group", "cid-1", "--record-id", "0", "--applicant", "user-1", "--inviter", "user-2", "--status", "AuditApprove"},
		{"group", "audit-join-validation", "--group", "cid-1", "--record-id", "not-an-id", "--applicant", "user-1", "--inviter", "user-2", "--status", "AuditApprove"},
		{"group", "audit-join-validation", "--group", "cid-1", "--record-id", "1", "--applicant", "user-1", "--inviter", "user-2", "--status", "AuditRefuse"},
		{"group", "audit-join-validation", "--group", " ", "--record-id", "1", "--applicant", "user-1", "--inviter", "user-2", "--status", "AuditApprove"},
	} {
		caller := &scriptedToolCaller{}
		err := runChatCoverageCommand(t, caller, args...)
		if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
			t.Fatalf("execute %v error=%T %v exit=%d, want typed validation", args, err, err, apperrors.ExitCode(err))
		}
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Reason == "confirmation_required" {
			t.Fatalf("execute %v error=%T %v, want input validation before confirmation", args, err, err)
		}
		if caller.calls != 0 {
			t.Fatalf("execute %v made %d remote calls before validation", args, caller.calls)
		}
	}
}

func TestChatFinalExclusionsRequireConfirmation(t *testing.T) {
	for _, args := range [][]string{
		{"chmod", "chat.message:send", "--conversation-id", "cid-1"},
		{"data-auth", "cross-org", "--target-org-id", "org-1"},
		{"clear-messages", "--conversation-id", "cid-1"},
		{"group", "audit-join-validation", "--group", "cid-1", "--record-id", "1", "--applicant", "user-1", "--inviter", "user-2", "--status", "AuditApprove"},
	} {
		caller := &scriptedToolCaller{}
		err := runChatCoverageCommand(t, caller, args...)
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Category != apperrors.CategoryValidation || typed.Reason != "confirmation_required" {
			t.Fatalf("execute %v error=%T %v, want confirmation_required", args, err, err)
		}
		if caller.calls != 0 {
			t.Fatalf("execute %v made %d remote calls before confirmation", args, caller.calls)
		}
	}
}

func TestChatFinalExclusionsDryRunNeverCallsRemoteOrRequiresYes(t *testing.T) {
	for _, args := range [][]string{
		{"chmod", "chat.message:send", "--conversation-id", "cid-1", "--dry-run"},
		{"data-auth", "cross-org", "--all", "--dry-run"},
		{"clear-messages", "--conversation-id", "cid-1", "--dry-run"},
		{"group", "audit-join-validation", "--group", "cid-1", "--record-id", "1", "--applicant", "user-1", "--inviter", "user-2", "--status", "AuditDelete", "--dry-run"},
	} {
		caller := &scriptedToolCaller{dry: true}
		if err := runChatCoverageCommand(t, caller, args...); err != nil {
			t.Fatalf("dry-run %v: %v", args, err)
		}
		if caller.calls != 0 {
			t.Fatalf("dry-run %v made %d remote calls", args, caller.calls)
		}
	}
}
