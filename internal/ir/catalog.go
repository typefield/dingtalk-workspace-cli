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

package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

type Catalog struct {
	Products []CanonicalProduct `json:"products"`
}

type LifecycleInfo struct {
	DeprecatedBy        int    `json:"deprecated_by,omitempty"`
	DeprecationDate     string `json:"deprecation_date,omitempty"`
	MigrationURL        string `json:"migration_url,omitempty"`
	DeprecatedCandidate bool   `json:"deprecated_candidate,omitempty"`
}

type ProductCLIMetadata struct {
	Command  string   `json:"command,omitempty"`
	Group    string   `json:"group,omitempty"`
	Hidden   bool     `json:"hidden,omitempty"`
	Skip     bool     `json:"skip,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
	Prefixes []string `json:"prefixes,omitempty"`
}

type CLIFlagHint struct {
	Shorthand string `json:"shorthand,omitempty"`
	Alias     string `json:"alias,omitempty"`
}

// FlagOverlay carries CLI-layer transformation metadata for a single
// MCP parameter: the flag alias the user types, the transform applied
// before dispatch, env-var fallback, default value, and whether the flag
// is hidden from help. Sourced from market.CLIToolOverride.Flags.
type FlagOverlay struct {
	Alias         string         `json:"alias,omitempty"`
	Transform     string         `json:"transform,omitempty"`
	TransformArgs map[string]any `json:"transform_args,omitempty"`
	EnvDefault    string         `json:"env_default,omitempty"`
	Default       string         `json:"default,omitempty"`
	Description   string         `json:"description,omitempty"`
	Hidden        bool           `json:"hidden,omitempty"`
}

// ToolAnnotations mirrors MCP 2025+ tool annotations. All hints are
// nullable: absence means "unknown", not "false". Populate only when the
// source has a clear signal — don't guess.
type ToolAnnotations struct {
	DestructiveHint *bool `json:"destructive_hint,omitempty"`
	ReadOnlyHint    *bool `json:"read_only_hint,omitempty"`
	IdempotentHint  *bool `json:"idempotent_hint,omitempty"`
	OpenWorldHint   *bool `json:"open_world_hint,omitempty"`
}

type CanonicalProduct struct {
	ID                        string              `json:"id"`
	DisplayName               string              `json:"display_name"`
	Description               string              `json:"description,omitempty"`
	ServerKey                 string              `json:"server_key"`
	Endpoint                  string              `json:"endpoint"`
	SchemaURI                 string              `json:"schema_uri,omitempty"`
	NegotiatedProtocolVersion string              `json:"negotiated_protocol_version,omitempty"`
	Source                    string              `json:"source,omitempty"`
	Degraded                  bool                `json:"degraded"`
	Lifecycle                 *LifecycleInfo      `json:"lifecycle,omitempty"`
	CLI                       *ProductCLIMetadata `json:"cli,omitempty"`
	Tools                     []ToolDescriptor    `json:"tools"`
}

type ToolDescriptor struct {
	RPCName         string                 `json:"rpc_name"`
	CLIName         string                 `json:"cli_name,omitempty"`
	Group           string                 `json:"group,omitempty"`
	Title           string                 `json:"title,omitempty"`
	Description     string                 `json:"description,omitempty"`
	InputSchema     map[string]any         `json:"input_schema,omitempty"`
	OutputSchema    map[string]any         `json:"output_schema,omitempty"`
	Sensitive       bool                   `json:"sensitive"`
	Auth            *ToolAuthMetadata      `json:"auth,omitempty"`
	Annotations     *ToolAnnotations       `json:"annotations,omitempty"`
	Hidden          bool                   `json:"hidden,omitempty"`
	FlagHints       map[string]CLIFlagHint `json:"flag_hints,omitempty"`
	FlagOverlay     map[string]FlagOverlay `json:"flag_overlay,omitempty"`
	SourceServerKey string                 `json:"source_server_key"`
	CanonicalPath   string                 `json:"canonical_path"`
}

type ToolAuthMetadata struct {
	Version              string   `json:"version,omitempty"`
	ProductCode          string   `json:"productCode,omitempty"`
	Domain               string   `json:"domain,omitempty"`
	ClientObservedScopes []string `json:"clientObservedScopes,omitempty"`
	RequiredScopes       []string `json:"requiredScopes,omitempty"`
	RequiredPermissions  []string `json:"requiredPermissions,omitempty"`
	RecommendedScopes    []string `json:"recommendedScopes,omitempty"`
	ExcludedScopes       []string `json:"excludedScopes,omitempty"`
	GrantProductCodes    []string `json:"grantProductCodes,omitempty"`
	RiskHint             string   `json:"riskHint,omitempty"`
	RiskAction           string   `json:"riskAction,omitempty"`
	ConfirmationRequired bool     `json:"confirmationRequired,omitempty"`
	Identities           []string `json:"identities,omitempty"`
	Source               string   `json:"source,omitempty"`
	AuthMetaVersion      string   `json:"authMetaVersion,omitempty"`
	AuthMetaHash         string   `json:"authMetaHash,omitempty"`
}

func extractToolAuthMetadata(schema map[string]any, productID string) *ToolAuthMetadata {
	raw := any(nil)
	if len(schema) > 0 {
		raw = schema["x-dingtalk-auth"]
	}
	if raw == nil {
		return productGrantAuthMetadata(productID)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var metadata ToolAuthMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil
	}
	if !hasMeaningfulToolAuthMetadata(metadata) {
		return nil
	}
	return &metadata
}

func hasMeaningfulToolAuthMetadata(metadata ToolAuthMetadata) bool {
	return strings.TrimSpace(metadata.Version) != "" ||
		strings.TrimSpace(metadata.ProductCode) != "" ||
		strings.TrimSpace(metadata.Domain) != "" ||
		len(metadata.ClientObservedScopes) > 0 ||
		len(metadata.RequiredScopes) > 0 ||
		len(metadata.RequiredPermissions) > 0 ||
		len(metadata.RecommendedScopes) > 0 ||
		len(metadata.ExcludedScopes) > 0 ||
		len(metadata.GrantProductCodes) > 0 ||
		strings.TrimSpace(metadata.RiskHint) != "" ||
		strings.TrimSpace(metadata.RiskAction) != "" ||
		metadata.ConfirmationRequired ||
		len(metadata.Identities) > 0 ||
		strings.TrimSpace(metadata.Source) != "" ||
		strings.TrimSpace(metadata.AuthMetaVersion) != "" ||
		strings.TrimSpace(metadata.AuthMetaHash) != ""
}

func productGrantAuthMetadata(productID string) *ToolAuthMetadata {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil
	}
	metadata := ToolAuthMetadata{
		Version:     "v1",
		ProductCode: productID,
		Domain:      productID,
		GrantProductCodes: []string{
			productID,
		},
		Source:          "dws-product-fallback",
		AuthMetaVersion: "v1",
	}
	metadata.AuthMetaHash = toolAuthMetaHash(metadata)
	return &metadata
}

func toolAuthMetaHash(metadata ToolAuthMetadata) string {
	metadata.AuthMetaHash = ""
	data, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (c Catalog) FindProduct(id string) (CanonicalProduct, bool) {
	for _, product := range c.Products {
		if product.ID == id {
			return product, true
		}
	}
	return CanonicalProduct{}, false
}

func (c Catalog) FindTool(path string) (CanonicalProduct, ToolDescriptor, bool) {
	productID, toolName, ok := strings.Cut(path, ".")
	if !ok || productID == "" || toolName == "" {
		return CanonicalProduct{}, ToolDescriptor{}, false
	}
	product, ok := c.FindProduct(productID)
	if !ok {
		return CanonicalProduct{}, ToolDescriptor{}, false
	}
	tool, ok := product.FindTool(toolName)
	if !ok {
		return CanonicalProduct{}, ToolDescriptor{}, false
	}
	return product, tool, true
}

func (p CanonicalProduct) FindTool(name string) (ToolDescriptor, bool) {
	for _, tool := range p.Tools {
		if tool.RPCName == name {
			return tool, true
		}
	}
	return ToolDescriptor{}, false
}

func nextCanonicalProductID(key, displayName, endpoint, cliCommand string, usedIDs map[string]struct{}) string {
	base := canonicalProductID(key, displayName, endpoint, cliCommand)
	id := base
	if _, exists := usedIDs[id]; exists {
		id = fmt.Sprintf("%s-%s", base, shorten(key))
	}
	usedIDs[id] = struct{}{}
	return id
}

func canonicalProductID(key, displayName, endpoint, cliCommand string) string {
	for _, candidate := range []string{
		slugify(cliCommand),
		endpointSlug(endpoint),
		slugify(displayName),
		slugify(key),
	} {
		if candidate != "" {
			return canonicalProductAlias(candidate)
		}
	}
	return "srv-" + shorten(key)
}

func canonicalProductAlias(id string) string {
	switch id {
	case "table":
		return "aitable"
	default:
		return id
	}
}

func endpointSlug(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return slugify(parts[len(parts)-1])
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			continue
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func shorten(value string) string {
	if strings.TrimSpace(value) == "" {
		sum := sha256.Sum256([]byte("dws"))
		return hex.EncodeToString(sum[:])[:8]
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil
	}
	return cloned
}

func cloneFlagHints(value map[string]CLIFlagHint) map[string]CLIFlagHint {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]CLIFlagHint, len(value))
	for key, hint := range value {
		out[key] = hint
	}
	return out
}

func cloneFlagOverlay(value map[string]FlagOverlay) map[string]FlagOverlay {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]FlagOverlay, len(value))
	for key, overlay := range value {
		overlay.TransformArgs = cloneTransformArgs(overlay.TransformArgs)
		out[key] = overlay
	}
	return out
}

func cloneTransformArgs(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil
	}
	return cloned
}

// deriveAnnotations maps the coarse-grained Sensitive bool onto MCP 2025+
// annotation hints. Only DestructiveHint is populated — the other hints
// (read_only, idempotent, open_world) stay nil until the upstream tool
// manifest advertises them explicitly. Guessing from name prefixes
// ("list_", "get_") risks false signals for AI agents.
func deriveAnnotations(sensitive bool) *ToolAnnotations {
	if !sensitive {
		return nil
	}
	destructive := true
	return &ToolAnnotations{DestructiveHint: &destructive}
}
