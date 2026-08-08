package errors

import "testing"

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
