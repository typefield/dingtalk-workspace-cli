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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cobracmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/transport"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	mcpHTTPCommandRoot               = "connector"
	mcpHTTPCommandDiscoveryPath      = "/cli/discovery/mcp"
	mcpHTTPCommandDiscoveryEnv       = "DWS_MCP_DISCOVERY_BASE_URL"
	preAIHubBaseURL                  = "https://pre-aihub.dingtalk.com"
	mcpHTTPCommandDiscoveryPageSize  = 100
	mcpHTTPCommandDiscoveryMaxPages  = 100
	mcpHTTPCommandListTTL            = 10 * time.Minute
	mcpHTTPCommandListRefreshTimeout = 5 * time.Second
)

type mcpHTTPCommandCacheFile struct {
	FetchedAt time.Time                  `json:"fetchedAt"`
	Commands  []mcpHTTPCommandDescriptor `json:"commands"`
}

type mcpHTTPCommandDescriptor struct {
	Path        []string       `json:"path"`
	ProductID   string         `json:"productId,omitempty"`
	Tool        string         `json:"tool"`
	Endpoint    string         `json:"endpoint,omitempty"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
	Source      string         `json:"source,omitempty"`
}

type publishedMCPDiscoveryResponse struct {
	Success    *bool                     `json:"success"`
	Result     publishedMCPDiscoveryPage `json:"result"`
	ErrorCode  string                    `json:"errorCode,omitempty"`
	Code       string                    `json:"code,omitempty"`
	ErrCode    string                    `json:"errCode,omitempty"`
	Message    string                    `json:"message,omitempty"`
	ErrorMsg   string                    `json:"errorMsg,omitempty"`
	ErrMessage string                    `json:"errMessage,omitempty"`
}

type publishedMCPDiscoveryPage struct {
	Values      []publishedMCPService `json:"values"`
	CurrentPage int                   `json:"currentPage"`
	PageSize    int                   `json:"pageSize"`
	TotalCount  int                   `json:"totalCount"`
	TotalPages  int                   `json:"totalPages"`
}

type publishedMCPService struct {
	MCPID       json.Number `json:"mcpId"`
	ServerName  string      `json:"serverName,omitempty"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	MCPURL      string      `json:"mcpUrl"`
}

type mcpHTTPFlagSpec struct {
	PropertyName string
	FlagName     string
	Kind         string
	Description  string
	Required     bool
}

// registerDynamicMCPHTTPCommands attaches commands discovered from the Portal
// published-MCP endpoint. Existing static commands keep priority; remote
// commands with the same CLI path are skipped.
func registerDynamicMCPHTTPCommands(root *cobra.Command, runner executor.Runner, flags *GlobalFlags, rootCtx context.Context) {
	if root == nil || runner == nil {
		return
	}
	ensureMCPHTTPRefreshCommand(root)

	source, err := currentMCPHTTPCommandCacheSource()
	if err != nil {
		slog.Debug("mcp-http: discovery source unavailable", "error", err)
		return
	}

	commands, fresh := readMCPHTTPCommandCache(source)
	if !fresh && shouldRefreshMCPHTTPCommands(os.Args[1:]) {
		if refreshed, err := refreshMCPHTTPCommandCache(rootCtx); err == nil {
			commands = refreshed
		} else {
			slog.Debug("mcp-http: refresh failed, using stale cache if any", "error", err)
		}
	}
	if len(commands) == 0 {
		return
	}

	registerMCPHTTPRuntimeRoutes(commands)
	addMCPHTTPCommands(root, runner, flags, commands)
}

func ensureMCPHTTPRefreshCommand(root *cobra.Command) {
	connectorRoot := ensureMCPHTTPGroup(root, []string{mcpHTTPCommandRoot, "mcp"})
	if connectorRoot == nil {
		return
	}
	if commandChild(connectorRoot, "refresh") == nil {
		cmd := &cobra.Command{
			Use:               "refresh",
			Short:             "刷新远程 MCP 命令缓存",
			Args:              cobra.NoArgs,
			DisableAutoGenTag: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				source, err := currentMCPHTTPCommandCacheSource()
				if err != nil {
					return err
				}
				commands, err := refreshMCPHTTPCommandCache(cmd.Context())
				if err != nil {
					return err
				}
				payload := map[string]any{
					"success": true,
					"count":   len(commands),
					"ttl":     mcpHTTPCommandListTTL.String(),
					"source":  source,
				}
				return output.WriteCommandPayload(cmd, payload, output.FormatJSON)
			},
		}
		connectorRoot.AddCommand(cmd)
	}
	if commandChild(connectorRoot, "inspect") == nil {
		connectorRoot.AddCommand(newMCPHTTPInspectCommand())
	}
}

func newMCPHTTPInspectCommand() *cobra.Command {
	var endpoint string
	cmd := &cobra.Command{
		Use:               "inspect",
		Short:             "读取指定 MCP 地址的协议和工具元数据",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint = strings.TrimSpace(endpoint)
			if endpoint == "" {
				endpoint = strings.TrimSpace(os.Getenv(mcpdevEndpointEnv))
			}
			if endpoint == "" {
				return apperrors.NewValidation("--url is required (or set DINGTALK_MCPDEV_MCP_URL)")
			}

			client := transport.NewClient(nil).WithAuth(
				resolveRuntimeAuthToken(cmd.Context(), ""),
				resolveIdentityHeaders(),
			)
			initialized, err := client.Initialize(cmd.Context(), endpoint)
			if err != nil {
				return err
			}
			if err := client.NotifyInitialized(cmd.Context(), endpoint); err != nil {
				return err
			}
			tools, err := client.ListTools(cmd.Context(), endpoint)
			if err != nil {
				return err
			}

			payload := map[string]any{
				"success":    true,
				"endpoint":   transport.RedactURL(endpoint),
				"transport":  "streamable-http",
				"initialize": initialized,
				"toolCount":  len(tools.Tools),
				"tools":      tools.Tools,
			}
			return output.WriteCommandPayload(cmd, payload, output.FormatJSON)
		},
	}
	cmd.Flags().StringVar(&endpoint, "url", "", "MCP Streamable HTTP 地址")
	return cmd
}

func refreshMCPHTTPCommandCache(ctx context.Context) ([]mcpHTTPCommandDescriptor, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	portalBase, err := mcpHTTPDiscoveryBaseURL()
	if err != nil {
		return nil, err
	}
	source := mcpHTTPCommandCacheSource(portalBase)

	callCtx, cancel := context.WithTimeout(ctx, mcpHTTPCommandListRefreshTimeout)
	defer cancel()

	commands, err := fetchMCPHTTPCommandList(callCtx, portalBase)
	if err != nil {
		return nil, err
	}
	if err := writeMCPHTTPCommandCache(source, commands); err != nil {
		slog.Debug("mcp-http: failed to write cache", "error", err)
	}
	return commands, nil
}

func readMCPHTTPCommandCache(endpoint string) ([]mcpHTTPCommandDescriptor, bool) {
	path, err := mcpHTTPCommandCachePath(endpoint)
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var cache mcpHTTPCommandCacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}
	if len(cache.Commands) == 0 {
		return nil, false
	}
	commands := dedupeAndSortMCPHTTPCommands(cache.Commands)
	fresh := time.Since(cache.FetchedAt) >= 0 && time.Since(cache.FetchedAt) < mcpHTTPCommandListTTL
	return commands, fresh
}

func writeMCPHTTPCommandCache(endpoint string, commands []mcpHTTPCommandDescriptor) error {
	path, err := mcpHTTPCommandCachePath(endpoint)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), config.DirPerm); err != nil {
		return err
	}
	payload := mcpHTTPCommandCacheFile{
		FetchedAt: time.Now(),
		Commands:  dedupeAndSortMCPHTTPCommands(commands),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, config.FilePerm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func mcpHTTPCommandCachePath(endpoint string) (string, error) {
	key := mcpHTTPCommandCacheKey(endpoint)
	if key == "" {
		return "", fmt.Errorf("empty MCP command cache key")
	}
	return filepath.Join(config.DefaultConfigDir(), "cache", "mcp-http-commands", key+".json"), nil
}

func mcpHTTPCommandCacheKey(endpoint string) string {
	redacted := redactEndpointForCacheKey(endpoint)
	sum := sha256.Sum256([]byte(redacted))
	return hex.EncodeToString(sum[:])[:24]
}

func redactEndpointForCacheKey(endpoint string) string {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return strings.TrimSpace(endpoint)
	}
	query := u.Query()
	for _, key := range []string{"key", "token", "access_token"} {
		if _, ok := query[key]; ok {
			query.Set(key, "REDACTED")
		}
	}
	u.RawQuery = query.Encode()
	u.Fragment = ""
	return u.String()
}

func currentMCPHTTPCommandCacheSource() (string, error) {
	portalBase, err := mcpHTTPDiscoveryBaseURL()
	if err != nil {
		return "", err
	}
	return mcpHTTPCommandCacheSource(portalBase), nil
}

func mcpHTTPCommandCacheSource(portalBase string) string {
	if discoveryURL, err := mcpHTTPDiscoveryURL(portalBase, 1, mcpHTTPCommandDiscoveryPageSize, ""); err == nil {
		if parsed, parseErr := url.Parse(discoveryURL); parseErr == nil {
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return "published-mcp:" + strings.TrimRight(parsed.String(), "/")
		}
	}
	return "published-mcp:" + strings.TrimRight(strings.TrimSpace(portalBase), "/") + mcpHTTPCommandDiscoveryPath
}

func mcpHTTPDiscoveryBaseURL() (string, error) {
	if raw := strings.TrimSpace(os.Getenv(mcpHTTPCommandDiscoveryEnv)); raw != "" {
		return normalizeMCPHTTPDiscoveryBaseURL(raw)
	}

	raw := preAIHubBaseURL
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid published MCP discovery URL: %q", transport.RedactURL(raw))
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return normalizeMCPHTTPDiscoveryBaseURL(parsed.String())
}

func normalizeMCPHTTPDiscoveryBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid published MCP discovery URL: %q", transport.RedactURL(raw))
	}
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func fetchMCPHTTPCommandList(ctx context.Context, portalBase string) ([]mcpHTTPCommandDescriptor, error) {
	services, err := fetchPublishedMCPServices(ctx, portalBase)
	if err != nil {
		return nil, err
	}
	return mcpHTTPCommandsFromPublishedMCPs(ctx, services)
}

func fetchPublishedMCPServices(ctx context.Context, portalBase string) ([]publishedMCPService, error) {
	var services []publishedMCPService
	for page := 1; page <= mcpHTTPCommandDiscoveryMaxPages; page++ {
		result, err := fetchPublishedMCPDiscoveryPage(ctx, portalBase, page, mcpHTTPCommandDiscoveryPageSize, "")
		if err != nil {
			return nil, err
		}
		services = append(services, result.Values...)
		if result.TotalPages > 0 {
			if page >= result.TotalPages {
				break
			}
			continue
		}
		if len(result.Values) < mcpHTTPCommandDiscoveryPageSize {
			break
		}
	}
	return services, nil
}

func fetchPublishedMCPDiscoveryPage(ctx context.Context, portalBase string, page, pageSize int, keyword string) (publishedMCPDiscoveryPage, error) {
	listURL, err := mcpHTTPDiscoveryURL(portalBase, page, pageSize, keyword)
	if err != nil {
		return publishedMCPDiscoveryPage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return publishedMCPDiscoveryPage{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(transport.HeaderSource, transport.SourceValue)
	if token := strings.TrimSpace(resolveRuntimeAuthToken(ctx, "")); token != "" {
		req.Header.Set("x-user-access-token", token)
	}
	for key, value := range resolveIdentityHeaders() {
		if key != "" && value != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return publishedMCPDiscoveryPage{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, config.MaxResponseBodySize))
	if err != nil {
		return publishedMCPDiscoveryPage{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return publishedMCPDiscoveryPage{}, fmt.Errorf("GET %s returned %s: %s", transport.RedactURL(listURL), resp.Status, strings.TrimSpace(string(data)))
	}

	var decoded publishedMCPDiscoveryResponse
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return publishedMCPDiscoveryPage{}, err
	}
	if decoded.Success != nil && !*decoded.Success {
		code := firstNonEmptyString(decoded.ErrorCode, decoded.Code, decoded.ErrCode)
		message := firstNonEmptyString(decoded.Message, decoded.ErrorMsg, decoded.ErrMessage)
		if code == "" && message == "" {
			message = "published MCP discovery failed"
		}
		if code != "" && message != "" {
			return publishedMCPDiscoveryPage{}, fmt.Errorf("published MCP discovery failed: %s: %s", code, message)
		}
		return publishedMCPDiscoveryPage{}, fmt.Errorf("published MCP discovery failed: %s%s", code, message)
	}
	return decoded.Result, nil
}

func mcpHTTPDiscoveryURL(portalBase string, page, pageSize int, keyword string) (string, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = mcpHTTPCommandDiscoveryPageSize
	}
	u, err := url.Parse(strings.TrimSpace(portalBase))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid published MCP discovery URL: %q", transport.RedactURL(portalBase))
	}
	path := strings.TrimRight(u.Path, "/")
	if path == "" {
		path = mcpHTTPCommandDiscoveryPath
	} else if !strings.HasSuffix(path, mcpHTTPCommandDiscoveryPath) {
		path += mcpHTTPCommandDiscoveryPath
	}
	u.Path = path
	u.Fragment = ""
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("keyword", keyword)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func mcpHTTPCommandsFromPublishedMCPs(ctx context.Context, services []publishedMCPService) ([]mcpHTTPCommandDescriptor, error) {
	seen := map[string]bool{}
	var commands []mcpHTTPCommandDescriptor
	var firstErr error

	token := resolveRuntimeAuthToken(ctx, "")
	headers := resolveIdentityHeaders()
	client := transport.NewClient(nil).WithAuth(token, headers)

	for _, service := range services {
		mcpURL := strings.TrimSpace(service.MCPURL)
		if mcpURL == "" {
			continue
		}
		result, err := client.ListTools(ctx, mcpURL)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("tools/list failed for MCP %s (%s): %w", service.displayName(), transport.RedactURL(mcpURL), err)
			}
			slog.Debug("mcp-http: tools/list failed for published MCP", "mcpId", service.idString(), "name", service.Name, "endpoint", transport.RedactURL(mcpURL), "error", err)
			continue
		}

		for _, tool := range result.Tools {
			descriptors := mcpHTTPCommandsFromPublishedTool(service, mcpURL, tool)
			for _, descriptor := range descriptors {
				descriptor = normalizeMCPHTTPCommand(descriptor)
				if descriptor.Tool == "" || len(descriptor.Path) == 0 {
					continue
				}
				key := strings.Join(descriptor.Path, " ") + "\x00" + descriptor.Tool
				if seen[key] {
					continue
				}
				seen[key] = true
				commands = append(commands, descriptor)
			}
		}
	}
	sort.Slice(commands, func(i, j int) bool {
		return strings.Join(commands[i].Path, " ") < strings.Join(commands[j].Path, " ")
	})
	if len(commands) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return commands, nil
}

func mcpHTTPCommandsFromPublishedTool(service publishedMCPService, endpoint string, tool transport.ToolDescriptor) []mcpHTTPCommandDescriptor {
	toolSlug := kebabCaseMCPHTTP(tool.Name)
	if toolSlug == "" {
		toolSlug = "tool"
	}
	serviceSlug := kebabCaseMCPHTTP(service.commandName(tool))
	if serviceSlug == "" {
		serviceSlug = toolSlug
	}
	productID := service.productID()
	description := strings.TrimSpace(tool.Description)
	if description == "" {
		description = strings.TrimSpace(tool.Title)
	}
	if description == "" {
		description = strings.TrimSpace(service.Description)
	}
	base := mcpHTTPCommandDescriptor{
		ProductID:   productID,
		Tool:        strings.TrimSpace(tool.Name),
		Endpoint:    strings.TrimSpace(endpoint),
		Description: description,
		InputSchema: tool.InputSchema,
		Source:      mcpHTTPCommandDiscoveryPath,
	}
	if base.Tool == "" {
		return nil
	}

	fixed := base
	fixed.Path = []string{serviceSlug, toolSlug}
	if serviceSlug == toolSlug {
		fixed.Path = []string{toolSlug}
	}

	debug := base
	debug.Path = []string{mcpHTTPCommandRoot, "mcp", "published", serviceSlug, toolSlug}

	return []mcpHTTPCommandDescriptor{fixed, debug}
}

func publishedMCPServiceForID(mcpID string) (publishedMCPService, error) {
	mcpID = strings.TrimSpace(mcpID)
	if mcpID == "" {
		return publishedMCPService{}, fmt.Errorf("empty mcpId")
	}
	endpoint, err := publishedMCPURLForID(mcpID)
	if err != nil {
		return publishedMCPService{}, err
	}
	return publishedMCPService{
		MCPID:  json.Number(mcpID),
		MCPURL: endpoint,
	}, nil
}

func publishedMCPURLForID(mcpID string) (string, error) {
	base, err := mcpHTTPPublishedGatewayBaseURL()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(base, "/") + "/server/org-" + url.PathEscape(strings.TrimSpace(mcpID)), nil
}

func mcpHTTPPublishedGatewayBaseURL() (string, error) {
	portalBase, err := mcpHTTPDiscoveryBaseURL()
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(portalBase)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid published MCP discovery URL: %q", transport.RedactURL(portalBase))
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "pre-aihub.dingtalk.com":
		parsed.Host = strings.Replace(parsed.Host, parsed.Hostname(), "pre-mcp-gw.dingtalk.com", 1)
	case "aihub.dingtalk.com":
		parsed.Host = strings.Replace(parsed.Host, parsed.Hostname(), "mcp-gw.dingtalk.com", 1)
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (s publishedMCPService) idString() string {
	return strings.TrimSpace(s.MCPID.String())
}

func (s publishedMCPService) displayName() string {
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	if id := s.idString(); id != "" {
		return "mcp-" + id
	}
	return "published-mcp"
}

func (s publishedMCPService) commandName(tool transport.ToolDescriptor) string {
	if name := strings.TrimSpace(s.ServerName); name != "" {
		return name
	}
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	if name := strings.TrimSpace(tool.Name); name != "" {
		return name
	}
	return s.displayName()
}

func (s publishedMCPService) productID() string {
	if id := s.idString(); id != "" {
		return "published-mcp-" + id
	}
	return "published-mcp-" + mcpHTTPCommandCacheKey(s.MCPURL)
}

func parseMCPHTTPCommandList(data []byte) ([]mcpHTTPCommandDescriptor, error) {
	var root any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	items, ok := findMCPHTTPListItems(root)
	if !ok {
		return nil, fmt.Errorf("%s response did not contain a command list", mcpHTTPCommandDiscoveryPath)
	}

	seen := map[string]bool{}
	var commands []mcpHTTPCommandDescriptor
	for _, item := range items {
		for _, command := range mcpHTTPCommandsFromItem(item, nil) {
			command = normalizeMCPHTTPCommand(command)
			if command.Tool == "" || len(command.Path) == 0 {
				continue
			}
			key := strings.Join(command.Path, " ") + "\x00" + command.Tool
			if seen[key] {
				continue
			}
			seen[key] = true
			commands = append(commands, command)
		}
	}
	sort.Slice(commands, func(i, j int) bool {
		return strings.Join(commands[i].Path, " ") < strings.Join(commands[j].Path, " ")
	})
	return commands, nil
}

func findMCPHTTPListItems(v any) ([]any, bool) {
	switch typed := v.(type) {
	case []any:
		return typed, true
	case map[string]any:
		for _, key := range []string{"commands", "list", "items", "tools", "mcpList", "mcp_list"} {
			if items, ok := typed[key].([]any); ok {
				return items, true
			}
		}
		for _, key := range []string{"data", "result"} {
			if child, ok := typed[key]; ok {
				if items, found := findMCPHTTPListItems(child); found {
					return items, true
				}
			}
		}
		return []any{typed}, true
	default:
		return nil, false
	}
}

func mcpHTTPCommandsFromItem(item any, parent map[string]any) []mcpHTTPCommandDescriptor {
	obj, ok := item.(map[string]any)
	if !ok {
		return nil
	}
	merged := mergeMCPHTTPObjects(parent, obj)
	if tools, ok := obj["tools"].([]any); ok && len(tools) > 0 {
		basePath := splitMCPHTTPPath(firstMCPHTTPString(obj, "cliPath", "commandPath", "path", "command", "cmd"))
		if len(basePath) == 0 {
			baseName := firstMCPHTTPString(obj, "commandName", "displayName", "title", "name", "mcpName", "mcpId")
			if baseName != "" {
				basePath = []string{mcpHTTPCommandRoot, "mcp", kebabCaseMCPHTTP(baseName)}
			}
		}
		out := make([]mcpHTTPCommandDescriptor, 0, len(tools))
		for _, tool := range tools {
			toolObj, ok := tool.(map[string]any)
			if !ok {
				continue
			}
			childParent := mergeMCPHTTPObjects(merged, map[string]any{})
			if len(basePath) > 0 && firstMCPHTTPString(toolObj, "cliPath", "commandPath", "path", "command", "cmd") == "" {
				toolName := firstMCPHTTPString(toolObj, "tool", "toolName", "rpcName", "actionName", "name")
				childParent["path"] = strings.Join(append(append([]string{}, basePath...), kebabCaseMCPHTTP(toolName)), " ")
			}
			out = append(out, mcpHTTPCommandsFromItem(toolObj, childParent)...)
		}
		return out
	}
	return []mcpHTTPCommandDescriptor{mcpHTTPCommandFromObject(merged)}
}

func mergeMCPHTTPObjects(parent, child map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range parent {
		out[key] = value
	}
	for key, value := range child {
		out[key] = value
	}
	return out
}

func mcpHTTPCommandFromObject(obj map[string]any) mcpHTTPCommandDescriptor {
	path := splitMCPHTTPPath(firstMCPHTTPString(obj, "cliPath", "commandPath", "path", "command", "cmd"))
	tool := firstMCPHTTPString(obj, "tool", "toolName", "rpcName", "actionName", "name")
	endpoint := firstMCPHTTPString(obj, "endpoint", "mcpURL", "mcpUrl", "mcp_url", "serverUrl", "server_url")
	productID := firstMCPHTTPString(obj, "productId", "productID", "product", "serverKey", "mcpId")
	description := firstMCPHTTPString(obj, "description", "desc", "title")
	if len(path) == 0 {
		path = []string{mcpHTTPCommandRoot, "mcp", kebabCaseMCPHTTP(tool)}
	}
	return mcpHTTPCommandDescriptor{
		Path:        path,
		ProductID:   productID,
		Tool:        tool,
		Endpoint:    endpoint,
		Description: description,
		InputSchema: firstMCPHTTPSchema(obj),
		Source:      mcpHTTPCommandDiscoveryPath,
	}
}

func normalizeMCPHTTPCommand(command mcpHTTPCommandDescriptor) mcpHTTPCommandDescriptor {
	command.Tool = strings.TrimSpace(command.Tool)
	command.Endpoint = strings.TrimSpace(command.Endpoint)
	command.ProductID = strings.TrimSpace(command.ProductID)
	if command.ProductID == "" {
		if command.Endpoint != "" {
			command.ProductID = "mcp-http-" + mcpHTTPCommandCacheKey(command.Endpoint)
		} else {
			command.ProductID = mcpdevProductID
		}
	}

	path := command.Path[:0]
	for _, token := range command.Path {
		token = strings.Trim(strings.TrimSpace(token), ". /")
		if token != "" && token != "dws" {
			path = append(path, token)
		}
	}
	if len(path) == 0 {
		path = []string{mcpHTTPCommandRoot, "mcp", kebabCaseMCPHTTP(command.Tool)}
	}
	if len(path) >= 2 && path[0] == "connect" && path[1] == "mcp" {
		path[0] = mcpHTTPCommandRoot
	}
	command.Path = path
	return command
}

func firstMCPHTTPSchema(obj map[string]any) map[string]any {
	for _, key := range []string{"inputSchema", "input_schema", "schema"} {
		if schema, ok := obj[key].(map[string]any); ok {
			return schema
		}
	}
	if params, ok := obj["parameters"].(map[string]any); ok {
		return mcpHTTPParametersToSchema(params)
	}
	return nil
}

func mcpHTTPParametersToSchema(params map[string]any) map[string]any {
	properties := map[string]any{}
	var required []any
	for key, raw := range params {
		prop, _ := raw.(map[string]any)
		if prop == nil {
			prop = map[string]any{"type": "string"}
		}
		properties[key] = prop
		if v, _ := prop["required"].(bool); v {
			required = append(required, key)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func firstMCPHTTPString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key]; ok {
			if s := stringifyMCPHTTPValue(value); s != "" {
				return s
			}
		}
	}
	return ""
}

func stringifyMCPHTTPValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func splitMCPHTTPPath(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '.' || r == '/' || r == '\t' || r == '\n' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" && part != "dws" {
			out = append(out, part)
		}
	}
	return out
}

func addMCPHTTPCommands(root *cobra.Command, runner executor.Runner, flags *GlobalFlags, commands []mcpHTTPCommandDescriptor) {
	for _, descriptor := range commands {
		if len(descriptor.Path) == 0 || descriptor.Tool == "" {
			continue
		}
		if findCommandByPath(root, descriptor.Path) != nil {
			continue
		}
		parent := ensureMCPHTTPGroup(root, descriptor.Path[:len(descriptor.Path)-1])
		if parent == nil {
			continue
		}
		parent.AddCommand(newMCPHTTPDynamicCommand(runner, flags, descriptor))
	}
}

func newMCPHTTPDynamicCommand(runner executor.Runner, flags *GlobalFlags, descriptor mcpHTTPCommandDescriptor) *cobra.Command {
	name := descriptor.Path[len(descriptor.Path)-1]
	specs := mcpHTTPFlagSpecs(descriptor.InputSchema)
	cmd := &cobra.Command{
		Use:               name,
		Short:             mcpHTTPCommandShort(descriptor),
		Long:              mcpHTTPCommandLong(descriptor, specs),
		Example:           mcpHTTPCommandExample(descriptor, specs),
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if mcpHTTPCommandRequiresWriteGuard(descriptor) && !mcpHTTPCommandDryRun(cmd) && !mcpHTTPCommandYes(cmd) {
				return apperrors.NewValidation(fmt.Sprintf("%s 是远程 MCP 写操作；加 --dry-run 预览，或确认后加 --yes 执行", cmd.CommandPath()))
			}
			params, err := collectMCPHTTPCommandParams(cmd, descriptor.InputSchema)
			if err != nil {
				return err
			}
			invocation := executor.NewHelperInvocation(
				cobracmd.LegacyCommandPath(cmd),
				descriptor.ProductID,
				descriptor.Tool,
				params,
			)
			invocation.DryRun = mcpHTTPCommandDryRun(cmd)
			result, err := runner.Run(cmd.Context(), invocation)
			if err != nil {
				return err
			}
			if flags != nil && flags.DryRun {
				result.Invocation.DryRun = true
			}
			return output.WriteCommandPayload(cmd, result, output.FormatJSON)
		},
	}
	annotateMCPHTTPDynamicCommand(cmd, descriptor)
	addMCPHTTPCommandFlags(cmd, specs)
	return cmd
}

func mcpHTTPCommandShort(descriptor mcpHTTPCommandDescriptor) string {
	short := strings.TrimSpace(descriptor.Description)
	if short == "" {
		short = strings.TrimSpace(descriptor.Tool)
	}
	short = strings.ReplaceAll(short, "\n", " ")
	short = strings.Join(strings.Fields(short), " ")
	if len([]rune(short)) > 96 {
		runes := []rune(short)
		short = string(runes[:96]) + "..."
	}
	return short
}

func mcpHTTPCommandLong(descriptor mcpHTTPCommandDescriptor, specs []mcpHTTPFlagSpec) string {
	var b strings.Builder
	if description := strings.TrimSpace(descriptor.Description); description != "" {
		b.WriteString(description)
		b.WriteString("\n\n")
	}
	b.WriteString("Dynamic DWS command generated from a published MCP tool.\n")
	b.WriteString("Use --format json for agent-readable stdout. corpId/userId and MCP credentials are supplied by the current DWS login; do not pass identity or secret parameters.\n\n")
	b.WriteString("Pass inputs either as individual flags or as --params '<JSON object>'. Individual flags override matching keys from --params.\n\n")
	b.WriteString("Command path: ")
	b.WriteString(mcpHTTPCommandDisplayPath(descriptor.Path))
	b.WriteString("\nMCP tool: ")
	b.WriteString(strings.TrimSpace(descriptor.Tool))
	b.WriteString("\nOutput: JSON written to stdout.\n\n")
	b.WriteString("Inputs:\n")
	if len(specs) == 0 {
		b.WriteString("  none\n")
		return b.String()
	}
	for _, spec := range specs {
		b.WriteString("  --")
		b.WriteString(spec.FlagName)
		b.WriteString(" (")
		b.WriteString(mcpHTTPFlagHelpKind(spec.Kind))
		if spec.Required {
			b.WriteString(", required")
		} else {
			b.WriteString(", optional")
		}
		b.WriteString(")")
		if spec.Description != "" {
			b.WriteString(": ")
			b.WriteString(strings.Join(strings.Fields(spec.Description), " "))
		}
		if spec.Kind == "json" {
			b.WriteString(" Pass object/array values as a JSON string.")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func mcpHTTPCommandExample(descriptor mcpHTTPCommandDescriptor, specs []mcpHTTPFlagSpec) string {
	parts := []string{mcpHTTPCommandDisplayPath(descriptor.Path)}
	added := false
	for _, spec := range specs {
		if !spec.Required {
			continue
		}
		parts = append(parts, mcpHTTPExampleFlag(spec)...)
		added = true
	}
	if !added && len(specs) > 0 {
		parts = append(parts, mcpHTTPExampleFlag(specs[0])...)
	}
	parts = append(parts, "--format", "json")
	examples := []string{"  " + strings.Join(parts, " ")}
	if params := mcpHTTPExampleParams(specs); params != "" {
		examples = append(examples, "  "+mcpHTTPCommandDisplayPath(descriptor.Path)+" --params "+params+" --format json")
	}
	return strings.Join(examples, "\n")
}

func mcpHTTPCommandDisplayPath(path []string) string {
	cleaned := make([]string, 0, len(path)+1)
	cleaned = append(cleaned, "dws")
	for _, part := range path {
		if part = strings.TrimSpace(part); part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, " ")
}

func mcpHTTPExampleFlag(spec mcpHTTPFlagSpec) []string {
	switch spec.Kind {
	case "boolean":
		return []string{"--" + spec.FlagName + "=true"}
	case "integer":
		return []string{"--" + spec.FlagName, "1"}
	case "number":
		return []string{"--" + spec.FlagName, "1.0"}
	case "string_array":
		return []string{"--" + spec.FlagName, "value1,value2"}
	case "integer_array":
		return []string{"--" + spec.FlagName, "1,2"}
	case "number_array":
		return []string{"--" + spec.FlagName, "1.0,2.0"}
	case "boolean_array":
		return []string{"--" + spec.FlagName, "true,false"}
	case "json":
		return []string{"--" + spec.FlagName, `'{"key":"value"}'`}
	default:
		value := mcpHTTPExampleStringValue(spec)
		if value == "" {
			value = "<value>"
		} else {
			value = mcpHTTPShellQuoteExample(value)
		}
		return []string{"--" + spec.FlagName, value}
	}
}

func mcpHTTPExampleParams(specs []mcpHTTPFlagSpec) string {
	params := map[string]any{}
	for _, spec := range specs {
		if spec.Required {
			params[spec.PropertyName] = mcpHTTPExampleJSONValue(spec)
		}
	}
	if len(params) == 0 && len(specs) > 0 {
		params[specs[0].PropertyName] = mcpHTTPExampleJSONValue(specs[0])
	}
	if len(params) == 0 {
		return ""
	}
	data, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	return mcpHTTPShellQuoteExample(string(data))
}

func mcpHTTPExampleJSONValue(spec mcpHTTPFlagSpec) any {
	switch spec.Kind {
	case "boolean":
		return true
	case "integer":
		return 1
	case "number":
		return 1.0
	case "string_array":
		return []string{"value1", "value2"}
	case "integer_array":
		return []int{1, 2}
	case "number_array":
		return []float64{1.0, 2.0}
	case "boolean_array":
		return []bool{true, false}
	case "json":
		return map[string]any{"key": "value"}
	default:
		if value := mcpHTTPExampleStringValue(spec); value != "" {
			return value
		}
		return "value"
	}
}

func mcpHTTPExampleStringValue(spec mcpHTTPFlagSpec) string {
	description := strings.TrimSpace(spec.Description)
	if description == "" {
		return ""
	}
	lower := strings.ToLower(description)
	markers := []string{"for example:", "example:", "e.g.", "eg:", "示例：", "示例:", "例如：", "例如:"}
	for _, marker := range markers {
		idx := strings.Index(lower, strings.ToLower(marker))
		if idx < 0 {
			continue
		}
		candidate := strings.TrimSpace(description[idx+len(marker):])
		candidate = strings.Trim(candidate, " \t\r\n\"'`“”‘’")
		for i, r := range candidate {
			switch r {
			case '\n', '\r', '。', '.', ';', '；', ',', '，', ')', '）':
				candidate = strings.TrimSpace(candidate[:i])
				candidate = strings.Trim(candidate, " \t\r\n\"'`“”‘’")
				return candidate
			}
		}
		return candidate
	}
	return ""
}

func mcpHTTPShellQuoteExample(value string) string {
	if value == "" || value == "<value>" {
		return value
	}
	for _, r := range value {
		if r == '_' || r == '-' || r == '.' || r == '/' || r == ':' {
			continue
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
	}
	return value
}

func mcpHTTPFlagHelpKind(kind string) string {
	switch kind {
	case "json":
		return "JSON string"
	case "string_array":
		return "string list"
	case "integer_array":
		return "integer list"
	case "number_array":
		return "number list"
	case "boolean_array":
		return "boolean list"
	case "":
		return "string"
	default:
		return kind
	}
}

func annotateMCPHTTPDynamicCommand(cmd *cobra.Command, descriptor mcpHTTPCommandDescriptor) {
	cmd.Annotations = map[string]string{
		"mcp-tool":   descriptor.Tool,
		"mcp-source": descriptor.ProductID,
	}
	if descriptor.Source != "" {
		cmd.Annotations["mcp-http-source"] = descriptor.Source
	}
}

func addMCPHTTPCommandFlags(cmd *cobra.Command, specs []mcpHTTPFlagSpec) {
	cmd.Flags().String("params", "", "Full MCP input JSON object. Individual field flags override matching keys. (type: JSON object)")
	for _, spec := range specs {
		usage := mcpHTTPFlagUsage(spec)
		switch spec.Kind {
		case "integer":
			cmd.Flags().Int(spec.FlagName, 0, usage)
		case "number":
			cmd.Flags().Float64(spec.FlagName, 0, usage)
		case "boolean":
			cmd.Flags().Bool(spec.FlagName, false, usage)
		case "string_array", "integer_array", "number_array", "boolean_array":
			cmd.Flags().StringSlice(spec.FlagName, nil, usage)
		default:
			cmd.Flags().String(spec.FlagName, "", usage)
		}
	}
}

func mcpHTTPFlagUsage(spec mcpHTTPFlagSpec) string {
	usage := strings.TrimSpace(spec.Description)
	if usage == "" {
		usage = fmt.Sprintf("Value for MCP input %s", spec.PropertyName)
	}
	usage = strings.Join(strings.Fields(usage), " ")
	var hints []string
	hints = append(hints, "type: "+mcpHTTPFlagHelpKind(spec.Kind))
	if spec.Required {
		hints = append(hints, "required")
	}
	if spec.Kind == "json" {
		hints = append(hints, "pass as JSON string")
	}
	if len(hints) > 0 {
		usage += " (" + strings.Join(hints, "; ") + ")"
	}
	return usage
}

func collectMCPHTTPCommandParams(cmd *cobra.Command, schema map[string]any) (map[string]any, error) {
	specs := mcpHTTPFlagSpecs(schema)
	params, err := mcpHTTPParamsFlagValue(cmd)
	if err != nil {
		return nil, err
	}
	for _, spec := range specs {
		flag := cmd.Flags().Lookup(spec.FlagName)
		if flag == nil || !flag.Changed {
			continue
		}
		value, err := mcpHTTPFlagValue(cmd, spec)
		if err != nil {
			return nil, err
		}
		params[spec.PropertyName] = value
	}
	if err := validateMCPHTTPRequiredParams(cmd, schema, specs, params); err != nil {
		return nil, err
	}
	return params, nil
}

func mcpHTTPParamsFlagValue(cmd *cobra.Command) (map[string]any, error) {
	params := map[string]any{}
	flag := cmd.Flags().Lookup("params")
	if flag == nil || !flag.Changed {
		return params, nil
	}
	raw, err := cmd.Flags().GetString("params")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, apperrors.NewValidation("--params must be a non-empty JSON object")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&params); err != nil {
		return nil, apperrors.NewValidation(fmt.Sprintf("--params must be a valid JSON object: %v", err))
	}
	return params, nil
}

func validateMCPHTTPRequiredParams(cmd *cobra.Command, schema map[string]any, specs []mcpHTTPFlagSpec, params map[string]any) error {
	required := mcpHTTPRequiredSet(schema)
	if len(required) == 0 {
		return nil
	}
	flagByProperty := map[string]string{}
	for _, spec := range specs {
		flagByProperty[spec.PropertyName] = spec.FlagName
	}
	keys := make([]string, 0, len(required))
	for key := range required {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, ok := params[key]
		if ok && value != nil {
			continue
		}
		if flag := flagByProperty[key]; flag != "" {
			return apperrors.NewValidation(fmt.Sprintf("missing required MCP input %q; pass --%s or include %q in --params", key, flag, key))
		}
		return apperrors.NewValidation(fmt.Sprintf("missing required MCP input %q; include it in --params", key))
	}
	return nil
}

func mcpHTTPFlagValue(cmd *cobra.Command, spec mcpHTTPFlagSpec) (any, error) {
	switch spec.Kind {
	case "integer":
		return cmd.Flags().GetInt(spec.FlagName)
	case "number":
		return cmd.Flags().GetFloat64(spec.FlagName)
	case "boolean":
		return cmd.Flags().GetBool(spec.FlagName)
	case "string_array":
		return cmd.Flags().GetStringSlice(spec.FlagName)
	case "integer_array":
		values, err := cmd.Flags().GetStringSlice(spec.FlagName)
		if err != nil {
			return nil, err
		}
		out := make([]int, 0, len(values))
		for _, value := range values {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(value))
			if parseErr != nil {
				return nil, apperrors.NewValidation(fmt.Sprintf("--%s expects integer values", spec.FlagName))
			}
			out = append(out, parsed)
		}
		return out, nil
	case "number_array":
		values, err := cmd.Flags().GetStringSlice(spec.FlagName)
		if err != nil {
			return nil, err
		}
		out := make([]float64, 0, len(values))
		for _, value := range values {
			parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if parseErr != nil {
				return nil, apperrors.NewValidation(fmt.Sprintf("--%s expects number values", spec.FlagName))
			}
			out = append(out, parsed)
		}
		return out, nil
	case "boolean_array":
		values, err := cmd.Flags().GetStringSlice(spec.FlagName)
		if err != nil {
			return nil, err
		}
		out := make([]bool, 0, len(values))
		for _, value := range values {
			parsed, parseErr := strconv.ParseBool(strings.TrimSpace(value))
			if parseErr != nil {
				return nil, apperrors.NewValidation(fmt.Sprintf("--%s expects boolean values", spec.FlagName))
			}
			out = append(out, parsed)
		}
		return out, nil
	case "json":
		raw, err := cmd.Flags().GetString(spec.FlagName)
		if err != nil {
			return nil, err
		}
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			return nil, apperrors.NewValidation(fmt.Sprintf("--%s must be valid JSON: %v", spec.FlagName, err))
		}
		return decoded, nil
	default:
		return cmd.Flags().GetString(spec.FlagName)
	}
}

func mcpHTTPFlagSpecs(schema map[string]any) []mcpHTTPFlagSpec {
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) == 0 {
		return nil
	}
	required := mcpHTTPRequiredSet(schema)
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	specs := make([]mcpHTTPFlagSpec, 0, len(keys))
	usedFlags := map[string]bool{}
	for _, key := range keys {
		prop, _ := properties[key].(map[string]any)
		kind := mcpHTTPFlagKind(prop)
		flag := kebabCaseMCPHTTP(key)
		if flag == "" || flag == "json" || flag == "params" || usedFlags[flag] {
			continue
		}
		usedFlags[flag] = true
		specs = append(specs, mcpHTTPFlagSpec{
			PropertyName: key,
			FlagName:     flag,
			Kind:         kind,
			Description:  firstMCPHTTPString(prop, "description", "title"),
			Required:     required[key],
		})
	}
	return specs
}

func mcpHTTPFlagKind(prop map[string]any) string {
	if prop == nil {
		return "string"
	}
	switch strings.TrimSpace(stringifyMCPHTTPValue(prop["type"])) {
	case "integer":
		return "integer"
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	case "object":
		return "json"
	case "array":
		items, _ := prop["items"].(map[string]any)
		switch strings.TrimSpace(stringifyMCPHTTPValue(items["type"])) {
		case "integer":
			return "integer_array"
		case "number":
			return "number_array"
		case "boolean":
			return "boolean_array"
		case "string":
			return "string_array"
		default:
			return "json"
		}
	default:
		return "string"
	}
}

func mcpHTTPRequiredSet(schema map[string]any) map[string]bool {
	out := map[string]bool{}
	switch raw := schema["required"].(type) {
	case []any:
		for _, value := range raw {
			if s, ok := value.(string); ok && s != "" {
				out[s] = true
			}
		}
	case []string:
		for _, value := range raw {
			if value != "" {
				out[value] = true
			}
		}
	}
	return out
}

func registerMCPHTTPRuntimeRoutes(commands []mcpHTTPCommandDescriptor) {
	dynamicMu.Lock()
	defer dynamicMu.Unlock()
	if dynamicEndpoints == nil {
		dynamicEndpoints = map[string]string{}
		dynamicProducts = map[string]bool{}
		dynamicAliases = map[string]string{}
		dynamicToolEndpoints = map[string]string{}
		registerDynamicServer(defaultPATServerDescriptor(), dynamicEndpoints, dynamicProducts, dynamicAliases, dynamicToolEndpoints)
	}
	for _, command := range commands {
		if command.Endpoint == "" {
			continue
		}
		if command.ProductID != "" {
			dynamicEndpoints[command.ProductID] = command.Endpoint
			dynamicProducts[command.ProductID] = true
		}
		if len(command.Path) > 0 {
			top := strings.TrimSpace(command.Path[0])
			if top != "" {
				dynamicEndpoints[top] = command.Endpoint
				dynamicProducts[top] = true
			}
		}
		if command.Tool != "" {
			dynamicToolEndpoints[command.Tool] = command.Endpoint
		}
	}
}

func shouldRefreshMCPHTTPCommands(args []string) bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DWS_MCP_HTTP_COMMAND_REFRESH")), "always") {
		return true
	}
	var tokens []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		tokens = append(tokens, splitMCPHTTPPath(arg)...)
	}
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == "dev" && tokens[i+1] == "mcp" {
			return true
		}
	}
	for _, token := range tokens {
		if token == mcpHTTPCommandRoot {
			return true
		}
	}
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == "schema" && tokens[i+1] == "dev" {
			return true
		}
	}
	return false
}

func maybeRefreshMCPHTTPCommandsAfterInvocation(ctx context.Context, invocation executor.Invocation, result executor.Result, flags *GlobalFlags) {
	if !shouldRefreshMCPHTTPCommandsAfterInvocation(invocation, flags) {
		return
	}
	if _, err := refreshMCPHTTPCommandCache(ctx); err != nil {
		slog.Debug("mcp-http: post-mutation refresh failed", "error", err)
	}
	if ok, err := refreshCurrentPublishedMCPCommandCache(ctx, invocation, result); err != nil {
		slog.Debug("mcp-http: current published MCP refresh failed", "error", err)
	} else if ok {
		slog.Debug("mcp-http: current published MCP refreshed", "mcpId", mcpHTTPInvocationMCPID(invocation, result))
	}
}

func shouldRefreshMCPHTTPCommandsAfterInvocation(invocation executor.Invocation, flags *GlobalFlags) bool {
	if flags != nil && flags.DryRun {
		return false
	}
	if invocation.DryRun {
		return false
	}
	return mcpDevToolMutatesCommands(invocation.Tool)
}

func mcpDevToolMutatesCommands(tool string) bool {
	switch strings.TrimSpace(tool) {
	case "mcp_service_create", "mcp_service_update", "mcp_service_delete",
		"mcp_tool_create", "mcp_tool_update", "mcp_tool_publish", "mcp_tool_delete":
		return true
	default:
		return false
	}
}

func refreshCurrentPublishedMCPCommandCache(ctx context.Context, invocation executor.Invocation, result executor.Result) (bool, error) {
	mcpID := mcpHTTPInvocationMCPID(invocation, result)
	if mcpID == "" {
		return false, nil
	}
	service, err := publishedMCPServiceForID(mcpID)
	if err != nil {
		return false, err
	}
	callCtx, cancel := context.WithTimeout(ctx, mcpHTTPCommandListRefreshTimeout)
	defer cancel()

	commands, err := mcpHTTPCommandsFromPublishedMCPs(callCtx, []publishedMCPService{service})
	if err != nil {
		return false, err
	}
	if len(commands) == 0 {
		return false, nil
	}

	source, err := currentMCPHTTPCommandCacheSource()
	if err != nil {
		return false, err
	}
	existing, _ := readMCPHTTPCommandCache(source)
	merged := mergeMCPHTTPCommandCacheForProduct(existing, commands, service.productID())
	if err := writeMCPHTTPCommandCache(source, merged); err != nil {
		return false, err
	}
	registerMCPHTTPRuntimeRoutes(commands)
	return true, nil
}

func mergeMCPHTTPCommandCacheForProduct(existing, replacement []mcpHTTPCommandDescriptor, productID string) []mcpHTTPCommandDescriptor {
	productID = strings.TrimSpace(productID)
	serviceSlug := existingMCPHTTPCommandServiceSlug(existing, productID)
	if serviceSlug != "" {
		for i := range replacement {
			replacement[i].Path = replaceMCPHTTPCommandServiceSlug(replacement[i].Path, serviceSlug)
		}
	}

	out := make([]mcpHTTPCommandDescriptor, 0, len(existing)+len(replacement))
	for _, command := range existing {
		if productID != "" && strings.TrimSpace(command.ProductID) == productID {
			continue
		}
		out = append(out, command)
	}
	out = append(out, replacement...)
	return dedupeAndSortMCPHTTPCommands(out)
}

func existingMCPHTTPCommandServiceSlug(commands []mcpHTTPCommandDescriptor, productID string) string {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return ""
	}
	for _, command := range commands {
		if strings.TrimSpace(command.ProductID) != productID {
			continue
		}
		if slug := mcpHTTPCommandServiceSlug(command.Path); slug != "" {
			return slug
		}
	}
	return ""
}

func mcpHTTPCommandServiceSlug(path []string) string {
	if len(path) >= 5 && path[0] == mcpHTTPCommandRoot && path[1] == "mcp" && path[2] == "published" {
		return strings.TrimSpace(path[3])
	}
	if len(path) >= 2 {
		return strings.TrimSpace(path[0])
	}
	return ""
}

func replaceMCPHTTPCommandServiceSlug(path []string, serviceSlug string) []string {
	serviceSlug = strings.TrimSpace(serviceSlug)
	if serviceSlug == "" {
		return append([]string{}, path...)
	}
	out := append([]string{}, path...)
	if len(out) >= 5 && out[0] == mcpHTTPCommandRoot && out[1] == "mcp" && out[2] == "published" {
		out[3] = serviceSlug
		return out
	}
	if len(out) >= 2 {
		out[0] = serviceSlug
		return out
	}
	if len(out) == 1 {
		return []string{serviceSlug, out[0]}
	}
	return out
}

func dedupeAndSortMCPHTTPCommands(commands []mcpHTTPCommandDescriptor) []mcpHTTPCommandDescriptor {
	seen := map[string]bool{}
	out := make([]mcpHTTPCommandDescriptor, 0, len(commands))
	for _, command := range commands {
		command = normalizeMCPHTTPCommand(command)
		if command.Tool == "" || len(command.Path) == 0 {
			continue
		}
		key := strings.Join(command.Path, " ") + "\x00" + command.Tool
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, command)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.Join(out[i].Path, " ") < strings.Join(out[j].Path, " ")
	})
	return out
}

func mcpHTTPInvocationMCPID(invocation executor.Invocation, result executor.Result) string {
	if value := mcpHTTPValueString(invocation.Params["mcpId"]); value != "" {
		return value
	}
	if value := mcpHTTPValueString(invocation.Params["mcp_id"]); value != "" {
		return value
	}
	for _, key := range []string{"mcpId", "mcp_id"} {
		if value := findMCPHTTPValueString(result.Response, key); value != "" {
			return value
		}
	}
	return ""
}

func findMCPHTTPValueString(value any, key string) string {
	switch typed := value.(type) {
	case map[string]any:
		if value := mcpHTTPValueString(typed[key]); value != "" {
			return value
		}
		for _, child := range typed {
			if value := findMCPHTTPValueString(child, key); value != "" {
				return value
			}
		}
	case []any:
		for _, child := range typed {
			if value := findMCPHTTPValueString(child, key); value != "" {
				return value
			}
		}
	}
	return ""
}

func mcpHTTPValueString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func mcpHTTPCommandRequiresWriteGuard(descriptor mcpHTTPCommandDescriptor) bool {
	parts := []string{descriptor.Tool}
	if len(descriptor.Path) > 0 {
		parts = append(parts, descriptor.Path[len(descriptor.Path)-1])
	}
	text := strings.ToLower(strings.Join(parts, " "))
	for _, marker := range []string{
		"create", "update", "delete", "publish", "debug", "submit",
		"enable", "disable", "add", "remove", "grant", "revoke",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func mcpHTTPCommandDryRun(cmd *cobra.Command) bool {
	return mcpHTTPBoolFlag(cmd, "dry-run")
}

func mcpHTTPCommandYes(cmd *cobra.Command) bool {
	return mcpHTTPBoolFlag(cmd, "yes")
}

func mcpHTTPBoolFlag(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	for _, flagSet := range cmdFlagSets(cmd) {
		if flagSet == nil || flagSet.Lookup(name) == nil {
			continue
		}
		value, err := flagSet.GetBool(name)
		if err == nil && value {
			return true
		}
	}
	return false
}

func cmdFlagSets(cmd *cobra.Command) []*pflag.FlagSet {
	if cmd == nil {
		return nil
	}
	var out []*pflag.FlagSet
	out = append(out, cmd.Flags(), cmd.InheritedFlags(), cmd.PersistentFlags())
	if root := cmd.Root(); root != nil {
		out = append(out, root.PersistentFlags())
	}
	return out
}

func findCommandByPath(root *cobra.Command, path []string) *cobra.Command {
	current := root
	for _, token := range path {
		if current == nil {
			return nil
		}
		current = commandChild(current, token)
	}
	return current
}

func ensureMCPHTTPGroup(root *cobra.Command, path []string) *cobra.Command {
	current := root
	for _, token := range path {
		if token == "" {
			return nil
		}
		next := commandChild(current, token)
		if next == nil {
			short := token
			if token == "published" {
				short = "已发布 MCP 动态命令"
			}
			next = &cobra.Command{
				Use:               token,
				Short:             short,
				Args:              cobra.NoArgs,
				TraverseChildren:  true,
				DisableAutoGenTag: true,
				RunE: func(cmd *cobra.Command, args []string) error {
					return cmd.Help()
				},
			}
			current.AddCommand(next)
		}
		current = next
	}
	return current
}

func commandChild(parent *cobra.Command, name string) *cobra.Command {
	if parent == nil {
		return nil
	}
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
		for _, alias := range child.Aliases {
			if alias == name {
				return child
			}
		}
	}
	return nil
}

func kebabCaseMCPHTTP(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if r == '_' || r == ' ' || r == '/' || r == '.' {
			b.WriteByte('-')
			continue
		}
		if unicode.IsUpper(r) {
			prevLowerOrDigit := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if i > 0 && (prevLowerOrDigit || nextLower) {
				b.WriteByte('-')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}
