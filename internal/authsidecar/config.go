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

package authsidecar

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/configmeta"
)

const (
	EnvAuthMode       = "DWS_AUTH_MODE"
	EnvSidecarAddress = "DWS_AUTH_SIDECAR_ADDR"
	EnvSidecarKeyID   = "DWS_AUTH_SIDECAR_KEY_ID"
	EnvSidecarKeyFile = "DWS_AUTH_SIDECAR_KEY_FILE"
	AuthModeSidecar   = "sidecar"
)

func init() {
	configmeta.Register(configmeta.ConfigItem{
		Name: EnvAuthMode, Category: configmeta.CategorySecurity,
		Description:  "认证模式；设为 sidecar 时要求 authsidecar 构建并禁止本地凭证回退",
		DefaultValue: "(本地认证)", Example: AuthModeSidecar,
	})
	configmeta.Register(configmeta.ConfigItem{
		Name: EnvSidecarAddress, Category: configmeta.CategoryNetwork,
		Description:  "同机认证 Sidecar 地址（优先 unix:///absolute/path.sock）",
		DefaultValue: "(未设置)", Example: "unix:///run/dws-sidecar/dws.sock",
	})
	configmeta.Register(configmeta.ConfigItem{
		Name: EnvSidecarKeyID, Category: configmeta.CategorySecurity,
		Description:  "Sidecar capability key 标识（不包含用户身份）",
		DefaultValue: "(未设置)", Example: "sandbox-agent-42",
	})
	configmeta.Register(configmeta.ConfigItem{
		Name: EnvSidecarKeyFile, Category: configmeta.CategorySecurity,
		Description:  "只读挂载的 Sidecar HMAC key 文件路径",
		DefaultValue: "(未设置)", Example: "/run/secrets/dws-sidecar.key",
	})
}

type Address struct {
	Network string
	Value   string
	URLHost string
}

type ClientConfig struct {
	Address Address
	KeyID   string
	Key     []byte
}

type ServerConfig struct {
	Version  int       `json:"version"`
	Bindings []Binding `json:"bindings"`
	Policies []Policy  `json:"policies"`
}

type Binding struct {
	KeyID     string    `json:"key_id"`
	KeyFile   string    `json:"key_file"`
	Profile   string    `json:"profile"`
	Policy    string    `json:"policy"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Enabled   bool      `json:"enabled"`
	key       []byte
}

type Policy struct {
	Name              string   `json:"name"`
	AllowedOrigins    []string `json:"allowed_origins"`
	AllowedPaths      []string `json:"allowed_paths"`
	AllowedTools      []string `json:"allowed_tools"`
	RequestsPerMinute int      `json:"requests_per_minute,omitempty"`
	MaxBodyBytes      int64    `json:"max_body_bytes,omitempty"`

	origins map[string]struct{}
	paths   map[string]struct{}
	tools   map[string]struct{}
}

func SidecarModeRequested() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvAuthMode)), AuthModeSidecar)
}

func ValidateAuthMode() error {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(EnvAuthMode)))
	if mode == "" || mode == AuthModeSidecar {
		return nil
	}
	return fmt.Errorf("unsupported %s value %q", EnvAuthMode, mode)
}

// ValidateSidecarEnvConsistency fails closed on half configuration: any
// sidecar client variable without DWS_AUTH_MODE=sidecar must abort instead of
// silently falling back to local credentials.
func ValidateSidecarEnvConsistency() error {
	var set []string
	for _, name := range []string{EnvSidecarAddress, EnvSidecarKeyID, EnvSidecarKeyFile} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			set = append(set, name)
		}
	}
	if len(set) == 0 || SidecarModeRequested() {
		return nil
	}
	return fmt.Errorf(
		"sidecar_config_incomplete: %s set without %s=%s; refusing to fall back to local credentials",
		strings.Join(set, ", "), EnvAuthMode, AuthModeSidecar,
	)
}

// ParseExactIdentitySelector accepts only a literal corpId:userId string. It
// rejects anything the looser profile resolution could reinterpret, such as
// surrounding whitespace or corpName:userName aliases that drift on rename.
func ParseExactIdentitySelector(selector string) (corpID, userID string, err error) {
	corpID, userID, ok := authpkg.ParseIdentitySelector(selector)
	if !ok || corpID+":"+userID != selector {
		return "", "", fmt.Errorf("profile must be a literal corpId:userId selector without whitespace")
	}
	return corpID, userID, nil
}

func ValidateClientArgs(args []string) error {
	for _, arg := range args {
		switch {
		case arg == "--token" || strings.HasPrefix(arg, "--token="):
			return fmt.Errorf("--token cannot be used with sidecar authentication")
		case arg == "--profile" || strings.HasPrefix(arg, "--profile="):
			return fmt.Errorf("--profile cannot be used with sidecar authentication; the trusted sidecar binds the profile")
		}
	}
	return nil
}

func ValidateCommandPath(commandPath string) error {
	if !SidecarModeRequested() {
		return nil
	}
	fields := strings.Fields(commandPath)
	if len(fields) < 2 {
		return nil
	}
	switch fields[1] {
	case "api", "audit", "auth", "cache", "catalog", "completion", "config", "dev", "doctor",
		"event", "markdown", "pat", "plugin", "profile", "recovery", "shortcut", "skill", "upgrade":
		return fmt.Errorf("sidecar_command_unsupported: %s is not available in the MCP-only sidecar MVP", fields[1])
	default:
		return nil
	}
}

func LoadClientConfigFromEnv() (ClientConfig, error) {
	if !SidecarModeRequested() {
		return ClientConfig{}, fmt.Errorf("%s must equal %q", EnvAuthMode, AuthModeSidecar)
	}
	address, err := ParseAddress(strings.TrimSpace(os.Getenv(EnvSidecarAddress)))
	if err != nil {
		return ClientConfig{}, fmt.Errorf("%s: %w", EnvSidecarAddress, err)
	}
	keyID := strings.TrimSpace(os.Getenv(EnvSidecarKeyID))
	if !validIdentifier(keyID) {
		return ClientConfig{}, fmt.Errorf("%s must contain 1-128 letters, digits, '.', '_' or '-'", EnvSidecarKeyID)
	}
	key, err := ReadKeyFile(strings.TrimSpace(os.Getenv(EnvSidecarKeyFile)))
	if err != nil {
		return ClientConfig{}, fmt.Errorf("%s: %w", EnvSidecarKeyFile, err)
	}
	return ClientConfig{Address: address, KeyID: keyID, Key: key}, nil
}

func ParseAddress(raw string) (Address, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Address{}, fmt.Errorf("address is empty")
	}
	if strings.HasPrefix(raw, "unix://") {
		path := strings.TrimPrefix(raw, "unix://")
		if path == "" || !filepath.IsAbs(path) {
			return Address{}, fmt.Errorf("unix socket path must be absolute")
		}
		return Address{Network: "unix", Value: filepath.Clean(path), URLHost: "dws-auth-sidecar"}, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Address{}, fmt.Errorf("parse address: %w", err)
	}
	if u.Scheme != "http" || u.User != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return Address{}, fmt.Errorf("address must be unix:///absolute/path.sock or same-host http://host:port without path, query or userinfo")
	}
	host := strings.ToLower(u.Hostname())
	if !sameHost(host) {
		return Address{}, fmt.Errorf("host %q is not loopback or an approved same-host alias", host)
	}
	if u.Port() == "" {
		return Address{}, fmt.Errorf("HTTP sidecar address requires an explicit port")
	}
	return Address{Network: "tcp", Value: u.Host, URLHost: u.Host}, nil
}

func sameHost(host string) bool {
	switch host {
	case "localhost", "host.docker.internal", "host.containers.internal", "host.lima.internal", "gateway.docker.internal":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func ReadKeyFile(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("key file path is empty")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat key file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("key file must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("key file permissions %04o are too broad; require 0600 or stricter", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	decoded, decodeErr := hex.DecodeString(trimmed)
	if decodeErr == nil && len(decoded) >= 32 {
		return decoded, nil
	}
	if len(data) < 32 {
		return nil, fmt.Errorf("key must contain at least 32 random bytes or 64 hex characters")
	}
	return append([]byte(nil), data...), nil
}

func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sidecar config: %w", err)
	}
	var config ServerConfig
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode sidecar config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("decode sidecar config: trailing JSON data")
	}
	if err := config.prepare(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *ServerConfig) prepare(baseDir string) error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported sidecar config version %d", c.Version)
	}
	policyNames := make(map[string]struct{}, len(c.Policies))
	for index := range c.Policies {
		policy := &c.Policies[index]
		if !validIdentifier(policy.Name) {
			return fmt.Errorf("policy[%d] has invalid name", index)
		}
		if _, duplicate := policyNames[policy.Name]; duplicate {
			return fmt.Errorf("duplicate policy %q", policy.Name)
		}
		policyNames[policy.Name] = struct{}{}
		if err := policy.prepare(); err != nil {
			return fmt.Errorf("policy %q: %w", policy.Name, err)
		}
	}
	keyIDs := make(map[string]struct{}, len(c.Bindings))
	for index := range c.Bindings {
		binding := &c.Bindings[index]
		if !validIdentifier(binding.KeyID) {
			return fmt.Errorf("binding[%d] has invalid key_id", index)
		}
		if _, duplicate := keyIDs[binding.KeyID]; duplicate {
			return fmt.Errorf("duplicate key_id %q", binding.KeyID)
		}
		keyIDs[binding.KeyID] = struct{}{}
		if _, ok := policyNames[binding.Policy]; !ok {
			return fmt.Errorf("binding %q references unknown policy %q", binding.KeyID, binding.Policy)
		}
		if _, _, err := ParseExactIdentitySelector(binding.Profile); err != nil {
			return fmt.Errorf("binding %q: %w", binding.KeyID, err)
		}
		keyFile := binding.KeyFile
		if !filepath.IsAbs(keyFile) {
			keyFile = filepath.Join(baseDir, keyFile)
		}
		key, err := ReadKeyFile(keyFile)
		if err != nil {
			return fmt.Errorf("binding %q key: %w", binding.KeyID, err)
		}
		binding.KeyFile = filepath.Clean(keyFile)
		binding.key = key
	}
	if len(c.Bindings) == 0 {
		return fmt.Errorf("sidecar config has no bindings")
	}
	return nil
}

func (p *Policy) prepare() error {
	if len(p.AllowedOrigins) == 0 {
		return fmt.Errorf("allowed_origins is empty")
	}
	p.origins = make(map[string]struct{}, len(p.AllowedOrigins))
	for _, origin := range p.AllowedOrigins {
		normalized, err := ValidateTargetOrigin(origin)
		if err != nil {
			return err
		}
		p.origins[normalized] = struct{}{}
	}
	p.tools = make(map[string]struct{}, len(p.AllowedTools))
	p.paths = make(map[string]struct{}, len(p.AllowedPaths))
	for _, allowedPath := range p.AllowedPaths {
		allowedPath = strings.TrimSpace(allowedPath)
		if allowedPath == "" || !strings.HasPrefix(allowedPath, "/") || strings.ContainsAny(allowedPath, "?#") {
			return fmt.Errorf("allowed_paths must contain absolute URL paths without query or fragment")
		}
		p.paths[allowedPath] = struct{}{}
	}
	if len(p.paths) == 0 {
		return fmt.Errorf("allowed_paths is empty")
	}
	for _, tool := range p.AllowedTools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			return fmt.Errorf("allowed_tools contains an empty tool")
		}
		p.tools[tool] = struct{}{}
	}
	if p.MaxBodyBytes <= 0 {
		p.MaxBodyBytes = 1 << 20
	}
	if p.RequestsPerMinute < 0 {
		return fmt.Errorf("requests_per_minute cannot be negative")
	}
	return nil
}

func ValidateTargetOrigin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse target origin: %w", err)
	}
	if u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("target origin %q must be https://host[:port] only", raw)
	}
	u.Scheme = "https"
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}
