package app

import (
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/audit"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/logging"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/transport"
)

var (
	auditActorOnce sync.Once
	cachedActor    audit.Actor
	cachedAgentID  string
)

func setupAuditSink() audit.Sink {
	return audit.BuildSink(defaultConfigDir())
}

func loadAuditIdentity() {
	auditActorOnce.Do(func() {
		configDir := defaultConfigDir()
		td, _ := auth.LoadTokenData(configDir)
		if td != nil {
			cachedActor = audit.Actor{
				UserID:   td.UserID,
				Name:     td.UserName,
				CorpID:   td.CorpID,
				CorpName: td.CorpName,
			}
		}
		id := auth.Load(configDir)
		if id != nil {
			cachedAgentID = id.AgentID
		}
	})
}

func emitAudit(sink audit.Sink, execID string, invokeStart time.Time, invocation executor.Invocation, endpoint string, retErr error, cliVersion string) {
	if sink == nil {
		return
	}
	if _, ok := sink.(audit.NopSink); ok {
		return
	}

	loadAuditIdentity()

	result := "success"
	var errCat, errReason string
	if retErr != nil {
		result = "error"
		errCat, errReason = classifyAuditError(retErr)
	}

	paramsSummary := logging.SanitizeArguments(invocation.Params, 1024)

	evt := &audit.Event{
		Timestamp:     invokeStart,
		ExecutionID:   execID,
		AgentID:       cachedAgentID,
		Actor:         cachedActor,
		Product:       invocation.CanonicalProduct,
		Command:       invocation.Tool,
		Endpoint:      transport.RedactURL(endpoint),
		ParamsSummary: paramsSummary,
		Result:        result,
		ErrCategory:   errCat,
		ErrReason:     errReason,
		DurationMs:    time.Since(invokeStart).Milliseconds(),
		CLIVersion:    cliVersion,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
	}

	_ = sink.Emit(evt)
}

func classifyAuditError(err error) (category, reason string) {
	if err == nil {
		return "", ""
	}
	var typed *apperrors.Error
	if errors.As(err, &typed) {
		return string(typed.Category), typed.Reason
	}
	return "unknown", err.Error()
}
