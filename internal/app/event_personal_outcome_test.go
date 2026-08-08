// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestEventStopOutcomeResultRetainsConfirmedAndUnknownStages(t *testing.T) {
	result, err := eventStopOutcomeResult(
		"event stop --as user: cancel subscription sub-2",
		[]string{"sub-1"},
		[]string{"sub-2"},
		"remote_subscription_cancel",
	)
	if err != nil {
		t.Fatalf("eventStopOutcomeResult: %v", err)
	}
	if err := output.ValidateResult(result); err != nil {
		t.Fatalf("ValidateResult: %v", err)
	}
	if result.Outcome() != output.OutcomePartialFailure || result.ExitCode() != 7 {
		t.Fatalf("result = outcome:%q exit:%d, want partial_failure/7", result.Outcome(), result.ExitCode())
	}
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatalf("EnvelopeFromResult: %v", err)
	}
	partial, ok := env.Data.(*output.PartialData)
	if !ok {
		t.Fatalf("data type = %T, want *PartialData", env.Data)
	}
	if partial.Total != 2 || len(partial.Succeeded) != 1 || len(partial.Failed) != 0 || len(partial.Unknown) != 1 {
		t.Fatalf("partial channels = %#v", partial)
	}
	succeeded, ok := partial.Succeeded[0].(map[string]any)
	if !ok || succeeded["id"] != "sub-1" || succeeded["state"] != "cancelled" {
		t.Fatalf("succeeded entry = %#v", partial.Succeeded[0])
	}
	if got := partial.Unknown[0]; got.ID != "sub-2" || got.Reason == "" {
		t.Fatalf("unknown entry = %#v", got)
	}
}

func TestEventStopOutcomeResultWithoutConfirmedSuccessIsFailure(t *testing.T) {
	result, err := eventStopOutcomeResult(
		"event stop --as user: cancel subscription sub-1",
		nil,
		[]string{"sub-1"},
		"remote_subscription_cancel",
	)
	if err != nil {
		t.Fatalf("eventStopOutcomeResult: %v", err)
	}
	if err := output.ValidateResult(result); err != nil {
		t.Fatalf("ValidateResult: %v", err)
	}
	if result.Outcome() != output.OutcomeFailure || result.ExitCode() != 1 {
		t.Fatalf("result = outcome:%q exit:%d, want failure/1", result.Outcome(), result.ExitCode())
	}
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatalf("EnvelopeFromResult: %v", err)
	}
	if env.Error == nil || env.Error.Subtype != string(apperrors.SubtypeEventStopUnverified) ||
		env.Error.Operation != "event.stop" || env.Error.Stage != "remote_subscription_cancel" {
		t.Fatalf("failure error = %#v", env.Error)
	}
}

func TestEventStopDualValidatePreservesLegacyError(t *testing.T) {
	cause := errors.New("control-plane response lost")
	err := eventStopPartialError(
		newEventStopCommand(),
		"event stop --as user: stop matching local consumer",
		cause,
		[]string{"sub-1"}, nil, "local_consumer_stop",
	)
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want original cause", err)
	}
	var apiErr *apperrors.Error
	if !errors.As(err, &apiErr) || apiErr.Reason != "partial_failure" {
		t.Fatalf("error = %T %#v, want legacy partial API error", err, apiErr)
	}
}

func TestEventStopStartsInDualValidation(t *testing.T) {
	if got := output.CommandRollout(newEventStopCommand()); got != output.RolloutDualValidate {
		t.Fatalf("event stop rollout = %q, want %q", got, output.RolloutDualValidate)
	}
}
