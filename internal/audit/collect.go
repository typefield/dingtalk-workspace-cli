package audit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/configmeta"
)

const (
	EnvAudit         = "DWS_AUDIT"
	EnvAuditDir      = "DWS_AUDIT_DIR"
	EnvRetentionDays = "DWS_AUDIT_RETENTION_DAYS"
	EnvForwardURL    = "DWS_AUDIT_FORWARD_URL"
	EnvForwardToken  = "DWS_AUDIT_FORWARD_TOKEN"
	EnvForwardRedact = "DWS_AUDIT_FORWARD_REDACT"

	defaultRetentionDays = 90
	auditSubdir          = "audit"
)

func init() {
	configmeta.Register(configmeta.ConfigItem{
		Name:         EnvAudit,
		Category:     configmeta.CategoryAudit,
		Description:  "操作审计日志开关（默认启用，设 0/false/off 关闭）",
		DefaultValue: "启用",
		Example:      "0",
	})
	configmeta.Register(configmeta.ConfigItem{
		Name:        EnvAuditDir,
		Category:    configmeta.CategoryAudit,
		Description: "审计日志目录（默认 <configDir>/audit）",
		Example:     "/var/log/dws-audit",
	})
	configmeta.Register(configmeta.ConfigItem{
		Name:         EnvRetentionDays,
		Category:     configmeta.CategoryAudit,
		Description:  "审计日志留存天数",
		DefaultValue: "90",
		Example:      "180",
	})
	configmeta.Register(configmeta.ConfigItem{
		Name:        EnvForwardURL,
		Category:    configmeta.CategoryAudit,
		Description: "审计事件远端转发 URL（POST JSON）",
		Example:     "https://siem.example.com/audit",
	})
	configmeta.Register(configmeta.ConfigItem{
		Name:        EnvForwardToken,
		Category:    configmeta.CategoryAudit,
		Description: "远端转发 Bearer Token",
		Sensitive:   true,
	})
	configmeta.Register(configmeta.ConfigItem{
		Name:         EnvForwardRedact,
		Category:     configmeta.CategoryAudit,
		Description:  "远端转发脱敏级别：none / hashed / minimal",
		DefaultValue: "none",
		Example:      "hashed",
	})
}

func IsEnabled() bool {
	v := os.Getenv(EnvAudit)
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no", "n":
		return false
	}
	return true
}

func BuildSink(configDir string) Sink {
	if !IsEnabled() {
		return NopSink{}
	}

	dir := os.Getenv(EnvAuditDir)
	if dir == "" {
		dir = filepath.Join(configDir, auditSubdir)
	}

	retention := defaultRetentionDays
	if v := os.Getenv(EnvRetentionDays); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			retention = n
		}
	}

	writer, err := NewDateRotatingWriter(dir, retention)
	if err != nil {
		return NopSink{}
	}

	chain := NewChain(dir)

	var forwarder *HTTPForwarder
	if fwdURL := os.Getenv(EnvForwardURL); fwdURL != "" {
		token := os.Getenv(EnvForwardToken)
		redact := RedactLevel(strings.ToLower(os.Getenv(EnvForwardRedact)))
		if redact != RedactHashed && redact != RedactMinimal {
			redact = RedactNone
		}
		forwarder = NewHTTPForwarder(fwdURL, token, redact)
	}

	return NewFileSink(writer, chain, forwarder)
}
