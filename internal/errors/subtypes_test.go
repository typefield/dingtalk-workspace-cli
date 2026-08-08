package errors

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSubtypeRegistryHasStableHighFrequencyDescriptors(t *testing.T) {
	tests := []struct {
		subtype  Subtype
		category Category
		retry    RetryPolicy
		action   bool
	}{
		{SubtypeMissingRequiredFlags, CategoryValidation, RetryNever, false},
		{SubtypeInvalidFlagValue, CategoryValidation, RetryNever, false},
		{SubtypeInvalidArgument, CategoryValidation, RetryNever, false},
		{SubtypeUnknownFlag, CategoryValidation, RetryNever, false},
		{SubtypeConfirmationRequired, CategoryValidation, RetryNever, true},
		{SubtypeRateLimit, CategoryAPI, RetryServerDirective, false},
		{SubtypePaginationInconsistent, CategoryAPI, RetryNever, false},
		{SubtypePaginationInvalid, CategoryValidation, RetryNever, false},
		{SubtypePaginationIncomplete, CategoryValidation, RetryNever, false},
		{SubtypePaginationConflict, CategoryValidation, RetryNever, false},
		{SubtypeChatSearchIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeChatListAllIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeFlagListIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeChatListIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeMessagesListDirectIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeChatMessagesIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeMyGroupsIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeThreadRepliesIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeProjectionUnknown, CategoryAPI, RetryNever, false},
		{SubtypeInputReadFailed, CategoryValidation, RetryNever, false},
		{SubtypeInvalidJSONInput, CategoryValidation, RetryNever, false},
		{SubtypeFormulaErrorsFound, CategoryValidation, RetryNever, false},
		{SubtypeDownloadOutputUnavailable, CategoryInternal, RetryNever, false},
		{SubtypeDownloadSizeMismatch, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeVersionNotFound, CategoryValidation, RetryNever, false},
		{SubtypeTargetTypeMismatch, CategoryValidation, RetryNever, false},
		{SubtypeTargetArgumentsConflict, CategoryValidation, RetryNever, false},
		{SubtypeMissingTarget, CategoryValidation, RetryNever, false},
		{SubtypeResolutionNotFound, CategoryValidation, RetryNever, false},
		{SubtypeResolutionAmbiguous, CategoryValidation, RetryNever, false},
		{SubtypeResolutionIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeResolutionBatchFailed, CategoryValidation, RetryNever, false},
		{SubtypeInvalidAITableURL, CategoryValidation, RetryNever, false},
		{SubtypeTargetNotFound, CategoryValidation, RetryNever, false},
		{SubtypeTargetAmbiguous, CategoryValidation, RetryNever, false},
		{SubtypeTargetTypeConflict, CategoryValidation, RetryNever, false},
		{SubtypeTargetIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeTargetInvalidResponse, CategoryAPI, RetryNever, false},
		{SubtypeTargetVerificationFailed, CategoryAPI, RetryNever, false},
		{SubtypeKeyValueConflict, CategoryValidation, RetryNever, false},
		{SubtypeAttachmentTokensUnavailable, CategoryValidation, RetryNever, false},
		{SubtypeUpstreamUnclassified, CategoryAPI, RetryNever, false},
		{SubtypeDiscoveryUpstreamUnclassified, CategoryDiscovery, RetryIdempotentReadOnly, false},
		{SubtypeUpstreamAuthenticationRequired, CategoryAuth, RetryNever, true},
		{SubtypeUpstreamAuthorizationDenied, CategoryAuth, RetryNever, true},
		{SubtypeToolProtocolIncompatible, CategoryDiscovery, RetryNever, false},
		{SubtypeBackendDependencyUnavailable, CategoryAPI, RetryServerDirective, false},
		{SubtypeUpstreamRequestRejected, CategoryAPI, RetryNever, false},
		{SubtypeBlockedFlag, CategoryValidation, RetryNever, true},
		{SubtypeAmbiguousFlag, CategoryValidation, RetryNever, true},
		{SubtypeSkillDownloadInfoUnavailable, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeDocCreateMissingNodeID, CategoryAPI, RetryNever, true},
		{SubtypeDocCreateInitialContentFailed, CategoryAPI, RetryNever, true},
		{SubtypeDocCheckpointUpdateFailed, CategoryAPI, RetryNever, true},
		{SubtypeDocCheckpointVerificationFailed, CategoryAPI, RetryNever, true},
		{SubtypeDocHistoryRevertVerificationFailed, CategoryAPI, RetryNever, true},
		{SubtypeEventStopUnverified, CategoryAPI, RetryNever, true},
		{SubtypeInvalidSuccessType, CategoryAPI, RetryNever, false},
		{SubtypeSkillSetupResultInvalid, CategoryInternal, RetryNever, false},
		{SubtypeSkillSetupFailed, CategoryInternal, RetryNever, false},
		{SubtypeBatchWriteFailed, CategoryAPI, RetryNever, true},
		{SubtypeDocGrantPermissionPartialFailure, CategoryAPI, RetryNever, true},
		{SubtypeDocShareMessageFailed, CategoryAPI, RetryNever, true},
		{SubtypeStdioInitializeError, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeStdioToolsListError, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeStdioError, CategoryAPI, RetryNever, false},
		{SubtypeMCPToolError, CategoryAPI, RetryNever, false},
		{SubtypeEmptyToolResponse, CategoryAPI, RetryNever, false},
		{SubtypePluginToolNotFound, CategoryValidation, RetryNever, false},
		{SubtypePluginInputSchemaInvalid, CategoryValidation, RetryNever, false},
		{SubtypeUnsupportedFormat, CategoryValidation, RetryNever, false},
		{SubtypeInvalidAgentCode, CategoryValidation, RetryNever, false},
		{SubtypeInvalidAgentHost, CategoryValidation, RetryNever, false},
		{SubtypeInvalidAgentProduct, CategoryValidation, RetryNever, false},
		{SubtypeAuthRefreshFailed, CategoryAuth, RetryNever, false},
		{SubtypeGatewayAuthExpired, CategoryAuth, RetryNever, true},
		{SubtypeNotAuthenticated, CategoryAuth, RetryNever, true},
		{SubtypeNotConfigured, CategoryAuth, RetryNever, true},
		{SubtypeRawAPICredentialsRequired, CategoryAuth, RetryNever, true},
		{SubtypeEndpointNotResolved, CategoryAPI, RetryNever, true},
		{SubtypeDocDownloadPreflightFailed, CategoryAPI, RetryNever, true},
		{SubtypeAmbiguousCommandFallback, CategoryValidation, RetryNever, true},
		{SubtypeIDIntersection, CategoryValidation, RetryNever, true},
		{SubtypeParameterConflict, CategoryValidation, RetryNever, false},
		{SubtypePATBatchRequiresYes, CategoryValidation, RetryNever, true},
		{SubtypeUnknownShortcut, CategoryValidation, RetryNever, true},
		{SubtypeUnknownSubcommand, CategoryValidation, RetryNever, true},
		{SubtypeUnsupportedAlidocExtension, CategoryValidation, RetryNever, true},
		{SubtypeAtMeIncomplete, CategoryAPI, RetryIdempotentReadOnly, false},
		{SubtypeThreadRootMessageNotFound, CategoryAPI, RetryNever, false},
		{SubtypeThreadContextMissing, CategoryAPI, RetryNever, false},
		{SubtypePATAuthTimeout, CategoryAuth, RetryNever, true},
		{SubtypePATAuthRejected, CategoryAuth, RetryNever, false},
		{SubtypePATAuthExpired, CategoryAuth, RetryNever, false},
		{SubtypePATAuthCancelled, CategoryAuth, RetryNever, false},
		{SubtypePersonalSubscriptionGuardFailed, CategoryInternal, RetryNever, false},
		{SubtypePersonalSubscriptionInvalid, CategoryValidation, RetryNever, false},
		{SubtypePersonalSubscriptionUnverified, CategoryAPI, RetryNever, true},
		{SubtypePersonalSubscriptionRejected, CategoryValidation, RetryNever, false},
		{SubtypePersonalSubscriptionAuth, CategoryAuth, RetryNever, true},
		{SubtypePartialFailure, CategoryAPI, RetryNever, true},
		{SubtypeSystemBusy, CategoryAPI, RetryNever, true},
		{SubtypeBusinessError, CategoryAPI, RetryNever, false},
		{SubtypeToolRequestBuildFailed, CategoryAPI, RetryNever, false},
		{SubtypeDiscoveryRequestBuildFailed, CategoryDiscovery, RetryNever, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.subtype), func(t *testing.T) {
			descriptor, ok := LookupSubtype(tt.subtype)
			if !ok {
				t.Fatalf("LookupSubtype(%q) missing", tt.subtype)
			}
			if descriptor.Subtype != tt.subtype || descriptor.Category != tt.category || descriptor.RetryPolicy != tt.retry || descriptor.RequireAction != tt.action || !descriptor.RequireHint || descriptor.DefaultHint == "" || descriptor.Description == "" {
				t.Fatalf("descriptor = %#v", descriptor)
			}
			if !IsRegisteredSubtype(string(tt.subtype)) {
				t.Fatalf("IsRegisteredSubtype(%q) = false", tt.subtype)
			}
		})
	}
	if IsRegisteredSubtype("upstream_unreviewed_reason") {
		t.Fatal("unreviewed reason unexpectedly registered")
	}
}

func TestWithSubtypePreservesLegacyReasonWire(t *testing.T) {
	err := NewValidation("missing name", WithSubtype(SubtypeMissingRequiredFlags))
	typed, ok := err.(*Error)
	if !ok {
		t.Fatalf("error = %T, want *Error", err)
	}
	if typed.Reason != string(SubtypeMissingRequiredFlags) || typed.Category != CategoryValidation || typed.ExitCode() != ExitCodeValidation || typed.Hint == "" {
		t.Fatalf("typed error = %#v", typed)
	}
}

func TestWithStableSubtypePreservesLegacyReasonWire(t *testing.T) {
	err := NewAPI("request cannot be built",
		WithStableSubtypeAndLegacyReason(SubtypeToolRequestBuildFailed, "request_build_failed"),
	)
	typed, ok := err.(*Error)
	if !ok || typed.Reason != "request_build_failed" || typed.StableSubtype != string(SubtypeToolRequestBuildFailed) {
		t.Fatalf("typed error = %#v", typed)
	}
	var rendered bytes.Buffer
	if printErr := PrintJSON(&rendered, err); printErr != nil {
		t.Fatalf("PrintJSON: %v", printErr)
	}
	var payload map[string]any
	if decodeErr := json.Unmarshal(rendered.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("decode: %v\n%s", decodeErr, rendered.String())
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok || errorPayload["reason"] != "request_build_failed" || errorPayload["subtype"] != string(SubtypeToolRequestBuildFailed) {
		t.Fatalf("legacy reason/stable subtype = %#v", payload)
	}
}

func TestWithSubtypeAllowsCommandSpecificHintToOverrideRegistryDefault(t *testing.T) {
	for name, options := range map[string][]Option{
		"hint after subtype":  {WithSubtype(SubtypeMissingRequiredFlags), WithHint("请传入 --name 后重试。")},
		"hint before subtype": {WithHint("请传入 --name 后重试。"), WithSubtype(SubtypeMissingRequiredFlags)},
	} {
		t.Run(name, func(t *testing.T) {
			err := NewValidation("missing name", options...)
			typed, ok := err.(*Error)
			if !ok {
				t.Fatalf("error = %T, want *Error", err)
			}
			if typed.Hint != "请传入 --name 后重试。" {
				t.Fatalf("hint = %q, want command-specific hint", typed.Hint)
			}
		})
	}
}
