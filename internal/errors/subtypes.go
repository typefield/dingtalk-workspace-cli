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

package errors

// Subtype is an approved machine-readable error reason. Only values present
// in the registry are stable Agent branch keys. The underlying Error.Reason
// remains a string during the gradual migration so legacy commands preserve
// their existing wire output.
type Subtype string

const (
	SubtypeMissingRequiredFlags      Subtype = "missing_required_flags"
	SubtypeInvalidFlagValue          Subtype = "invalid_flag_value"
	SubtypeInvalidArgument           Subtype = "invalid_argument"
	SubtypeUnknownFlag               Subtype = "unknown_flag"
	SubtypeConfirmationRequired      Subtype = "confirmation_required"
	SubtypeRateLimit                 Subtype = "rate_limit"
	SubtypePaginationInconsistent    Subtype = "pagination_inconsistent"
	SubtypeProjectionUnknown         Subtype = "projection_unknown"
	SubtypeInputReadFailed           Subtype = "input_read_failed"
	SubtypeInvalidJSONInput          Subtype = "invalid_json_input"
	SubtypeFormulaErrorsFound        Subtype = "formula_errors_found"
	SubtypeDownloadOutputUnavailable Subtype = "download_output_unavailable"
	SubtypeDownloadSizeMismatch      Subtype = "download_size_mismatch"
)

// RetryPolicy describes whether a descriptor can ever recommend replay. It
// does not cause the CLI to replay requests: retry decisions remain with the
// caller/Agent and must also consider idempotency and execution_started.
type RetryPolicy string

const (
	RetryNever              RetryPolicy = "never"
	RetryServerDirective    RetryPolicy = "server_directive"
	RetryIdempotentReadOnly RetryPolicy = "idempotent_read_only"
)

// SubtypeDescriptor is the registry entry for a public, stable subtype.
// Recovery text deliberately stays at the command boundary: a generic
// descriptor cannot safely invent a resource ID, credential scope, or action.
type SubtypeDescriptor struct {
	Subtype       Subtype
	Category      Category
	RetryPolicy   RetryPolicy
	RequireHint   bool
	RequireAction bool
	// DefaultHint is used only when a command has no more-specific recovery
	// hint. It must be safe without inventing resource IDs, credentials, or a
	// business-terminal result; command-local WithHint remains authoritative.
	DefaultHint string
	Description string
}

var subtypeRegistry = map[Subtype]SubtypeDescriptor{
	SubtypeMissingRequiredFlags: {
		Subtype:       SubtypeMissingRequiredFlags,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请补齐缺失的必填参数后重试；运行当前命令的 --help 查看参数说明。",
		Description:   "required command input is missing",
	},
	SubtypeInvalidFlagValue: {
		Subtype:       SubtypeInvalidFlagValue,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请根据当前命令的 --help 检查参数取值、格式和互斥关系，修正后再重试。",
		Description:   "a command flag value is invalid or conflicts with another flag",
	},
	SubtypeInvalidArgument: {
		Subtype:       SubtypeInvalidArgument,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请根据当前命令的 --help 检查输入参数和组合约束，修正后再重试。",
		Description:   "a command argument or local input combination is invalid",
	},
	SubtypeUnknownFlag: {
		Subtype:       SubtypeUnknownFlag,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请运行当前命令的 --help 查看可用参数，修正参数名后再重试。",
		Description:   "an unsupported command flag was supplied",
	},
	SubtypeConfirmationRequired: {
		Subtype:       SubtypeConfirmationRequired,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "请先使用 --dry-run 预览；获得用户确认后以相同参数追加 --yes。",
		Description:   "a protected write was stopped before request execution",
	},
	SubtypeRateLimit: {
		Subtype:       SubtypeRateLimit,
		Category:      CategoryAPI,
		RetryPolicy:   RetryServerDirective,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请按服务端给出的等待时间退避后重试；未提供时使用指数退避。",
		Description:   "the upstream service asked the caller to slow down",
	},
	SubtypePaginationInconsistent: {
		Subtype:       SubtypePaginationInconsistent,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请检查上游的分页游标和 hasMore 证据；确认前不要把结果当作完整。",
		Description:   "pagination evidence is incomplete or contradictory",
	},
	SubtypeProjectionUnknown: {
		Subtype:       SubtypeProjectionUnknown,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请记录脱敏响应形状并提交诊断；不要将该结果当作空集合。",
		Description:   "an upstream response cannot be safely projected",
	},
	SubtypeInputReadFailed: {
		Subtype:       SubtypeInputReadFailed,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请检查输入文件路径、读取权限或 stdin 内容后重新执行。",
		Description:   "a local input file or standard input stream could not be read",
	},
	SubtypeInvalidJSONInput: {
		Subtype:       SubtypeInvalidJSONInput,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请修正输入 JSON 的语法和字段形状后重新执行。",
		Description:   "a local JSON input could not be parsed",
	},
	SubtypeFormulaErrorsFound: {
		Subtype:       SubtypeFormulaErrorsFound,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请根据 details 中的错误位置和类型修正公式后，再重新校验。",
		Description:   "formula verification completed and found formula errors",
	},
	SubtypeDownloadOutputUnavailable: {
		Subtype:       SubtypeDownloadOutputUnavailable,
		Category:      CategoryInternal,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请检查本地输出路径、磁盘空间和文件权限；确认文件状态后再决定是否重试。",
		Description:   "a completed download could not be inspected at its local output path",
	},
	SubtypeDownloadSizeMismatch: {
		Subtype:       SubtypeDownloadSizeMismatch,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留当前文件供核查；确认服务端元数据后以新的输出路径重新下载。",
		Description:   "downloaded bytes do not match upstream file metadata",
	},
}

// LookupSubtype returns the immutable descriptor for an approved subtype.
func LookupSubtype(subtype Subtype) (SubtypeDescriptor, bool) {
	descriptor, ok := subtypeRegistry[subtype]
	return descriptor, ok
}

// IsRegisteredSubtype reports whether a value is an approved stable subtype.
// It is intentionally not used to reject legacy free-form reasons at runtime;
// doing so before each command is migrated would be a wire-breaking change.
func IsRegisteredSubtype(subtype string) bool {
	_, ok := LookupSubtype(Subtype(subtype))
	return ok
}

// WithSubtype records a registered, stable subtype. New production code must
// prefer this over WithReason("literal"); WithReason remains solely for
// compatibility and for values still under Agent review. It deliberately keeps
// the existing string-valued Reason, Category, and exit-code semantics. A
// registered descriptor may add a safe fallback hint; that additive recovery
// guidance is intentional, and an adjacent or later WithHint always replaces
// it with command-specific advice.
func WithSubtype(subtype Subtype) Option {
	return func(err *Error) {
		err.Reason = string(subtype)
		if err.Hint != "" {
			return
		}
		if descriptor, ok := LookupSubtype(subtype); ok {
			err.Hint = descriptor.DefaultHint
		}
	}
}
