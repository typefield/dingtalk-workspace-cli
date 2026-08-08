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
	SubtypeMissingRequiredFlags   Subtype = "missing_required_flags"
	SubtypeUnknownFlag            Subtype = "unknown_flag"
	SubtypeConfirmationRequired   Subtype = "confirmation_required"
	SubtypeRateLimit              Subtype = "rate_limit"
	SubtypePaginationInconsistent Subtype = "pagination_inconsistent"
	SubtypeProjectionUnknown      Subtype = "projection_unknown"
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
	Description   string
}

var subtypeRegistry = map[Subtype]SubtypeDescriptor{
	SubtypeMissingRequiredFlags: {
		Subtype:       SubtypeMissingRequiredFlags,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		Description:   "required command input is missing",
	},
	SubtypeUnknownFlag: {
		Subtype:       SubtypeUnknownFlag,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		Description:   "an unsupported command flag was supplied",
	},
	SubtypeConfirmationRequired: {
		Subtype:       SubtypeConfirmationRequired,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		Description:   "a protected write was stopped before request execution",
	},
	SubtypeRateLimit: {
		Subtype:       SubtypeRateLimit,
		Category:      CategoryAPI,
		RetryPolicy:   RetryServerDirective,
		RequireHint:   true,
		RequireAction: false,
		Description:   "the upstream service asked the caller to slow down",
	},
	SubtypePaginationInconsistent: {
		Subtype:       SubtypePaginationInconsistent,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		Description:   "pagination evidence is incomplete or contradictory",
	},
	SubtypeProjectionUnknown: {
		Subtype:       SubtypeProjectionUnknown,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		Description:   "an upstream response cannot be safely projected",
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
// compatibility and for values still under Agent review.
func WithSubtype(subtype Subtype) Option {
	return WithReason(string(subtype))
}
