package smart

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

func TestResolveSpaceContractAndSafety(t *testing.T) {
	if ResolveSpace.Service != "wiki" || ResolveSpace.Command != "+resolve-space" {
		t.Fatalf("unexpected identity: %s %s", ResolveSpace.Service, ResolveSpace.Command)
	}
	if ResolveSpace.Safety.Effect != "read" || ResolveSpace.Safety.Risk != "low" ||
		ResolveSpace.Safety.Confirmation != "not_required" || ResolveSpace.Safety.Idempotency != "idempotent" {
		t.Fatalf("resolve-space safety drift: %#v", ResolveSpace.Safety)
	}
	if ResolveSpace.Contract.Identity.CanonicalPath != "wiki.shortcut_resolve_space" ||
		ResolveSpace.Contract.Interface == nil || ResolveSpace.Contract.Selection.AgentSummary == "" {
		t.Fatalf("resolve-space contract incomplete: %#v", ResolveSpace.Contract)
	}
	if len(ResolveSpace.Flags) != 1 || ResolveSpace.Flags[0].Name != "name" || !ResolveSpace.Flags[0].Required {
		t.Fatalf("resolve-space flags drift: %#v", ResolveSpace.Flags)
	}
	if ResolveSpace.Risk != shortcut.RiskRead {
		t.Fatalf("resolve-space risk = %q, want read", ResolveSpace.Risk)
	}
	if ResolveSpace.Contract.Result == nil {
		t.Fatal("resolve-space result contract is missing")
	}
}

func resolveSpacePageFixture(hasMore bool, nextToken string, rows ...any) map[string]any {
	return map[string]any{
		"success":       true,
		"logId":         "log-redacted",
		"hasMore":       hasMore,
		"nextPageToken": nextToken,
		"wikiSpaces":    rows,
	}
}

func resolveSpaceRow(id, name string) map[string]any {
	return map[string]any{
		"workspaceId": id,
		"name":        name,
		"description": nil,
		"createTime":  nil,
		"updateTime":  nil,
		"spaceUrl":    "https://example.invalid/wiki",
	}
}

func TestResolveSpacePageProjectionUsesReviewedWorkspaceID(t *testing.T) {
	page, err := projectResolveSpacePage(resolveSpacePageFixture(false, "",
		resolveSpaceRow("workspace-1", "产品文档"),
		resolveSpaceRow("workspace-2", "产品文档-归档"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if page.hasMore || page.nextToken != "" || len(page.spaces) != 2 {
		t.Fatalf("page=%#v", page)
	}
	if page.spaces[0]["spaceId"] != "workspace-1" || page.spaces[0]["name"] != "产品文档" {
		t.Fatalf("spaces=%#v", page.spaces)
	}
}

func TestResolveSpacePageProjectionFailsClosed(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"unknown container": func(root map[string]any) {
			delete(root, "wikiSpaces")
			root["items"] = []any{}
		},
		"missing success":     func(root map[string]any) { delete(root, "success") },
		"non boolean success": func(root map[string]any) { root["success"] = "true" },
		"business failure":    func(root map[string]any) { root["success"] = false },
		"non boolean hasMore": func(root map[string]any) { root["hasMore"] = "false" },
		"continuation without token": func(root map[string]any) {
			root["hasMore"] = true
			root["nextPageToken"] = ""
		},
		"terminal with token": func(root map[string]any) { root["nextPageToken"] = "stale" },
		"invalid row": func(root map[string]any) {
			root["wikiSpaces"].([]any)[0] = "not-an-object"
		},
		"unknown row key": func(root map[string]any) {
			root["wikiSpaces"].([]any)[0].(map[string]any)["id"] = "wrong-id"
		},
		"missing workspaceId": func(root map[string]any) {
			delete(root["wikiSpaces"].([]any)[0].(map[string]any), "workspaceId")
		},
		"empty workspaceId": func(root map[string]any) {
			root["wikiSpaces"].([]any)[0].(map[string]any)["workspaceId"] = " "
		},
		"missing name": func(root map[string]any) {
			delete(root["wikiSpaces"].([]any)[0].(map[string]any), "name")
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := resolveSpacePageFixture(false, "", resolveSpaceRow("workspace-1", "产品文档"))
			mutate(fixture)
			if page, err := projectResolveSpacePage(fixture); err == nil || page.spaces != nil {
				t.Fatalf("page=%#v err=%v", page, err)
			}
		})
	}
}

type resolveSpaceProjectionCaller struct {
	calls  int
	params []map[string]any
	pages  []map[string]any
}

func (c *resolveSpaceProjectionCaller) CallTool(_ context.Context, product, tool string, params map[string]any) (*edition.ToolResult, error) {
	c.calls++
	if product != "wiki" || tool != "list_wikiSpaces" {
		return nil, stderrors.New("unexpected resolve-space route")
	}
	copyParams := make(map[string]any, len(params))
	for key, value := range params {
		copyParams[key] = value
	}
	c.params = append(c.params, copyParams)
	var payload map[string]any
	if len(c.pages) > 0 {
		if c.calls > len(c.pages) {
			return nil, stderrors.New("unexpected extra page request")
		}
		payload = c.pages[c.calls-1]
	} else {
		switch params["pageToken"] {
		case nil:
			payload = resolveSpacePageFixture(true, "page-2",
				resolveSpaceRow("workspace-1", "General"),
				resolveSpaceRow("workspace-2", "Product Handbook"),
			)
		case "page-2":
			payload = resolveSpacePageFixture(false, "",
				resolveSpaceRow("workspace-3", "Product Archive"),
			)
		default:
			return nil, stderrors.New("unexpected page token")
		}
	}
	raw, _ := json.Marshal(payload)
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: string(raw)}}}, nil
}

func (c *resolveSpaceProjectionCaller) Format() string { return "json" }
func (c *resolveSpaceProjectionCaller) DryRun() bool   { return false }
func (c *resolveSpaceProjectionCaller) Fields() string { return "" }
func (c *resolveSpaceProjectionCaller) JQ() string     { return "" }

func TestResolveSpaceDualValidationExhaustsPagesAndPreservesLegacyShape(t *testing.T) {
	declaration := ResolveSpace
	declaration.OutputRollout = output.RolloutDualValidate
	caller := &resolveSpaceProjectionCaller{}
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(declaration))
	cmd.PersistentFlags().String("format", "json", "")
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--name", "Product", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if caller.calls != 2 || stderr.Len() != 0 {
		t.Fatalf("calls=%d stderr=%q", caller.calls, stderr.String())
	}
	if caller.params[0]["wikiSpaceType"] != "orgWikiSpace" || caller.params[0]["pageSize"] != resolveSpacePageSize {
		t.Fatalf("first params=%#v", caller.params[0])
	}
	if caller.params[1]["pageToken"] != "page-2" {
		t.Fatalf("second params=%#v", caller.params[1])
	}
	want := "{\n  \"candidates\": [\n    {\n      \"name\": \"Product Handbook\",\n      \"spaceId\": \"workspace-2\"\n    },\n    {\n      \"name\": \"Product Archive\",\n      \"spaceId\": \"workspace-3\"\n    }\n  ],\n  \"count\": 2,\n  \"resolved\": false\n}\n"
	if stdout.String() != want {
		t.Fatalf("legacy bytes changed:\n%s", stdout.String())
	}
}

func TestResolveSpaceUnifiedResultUsesExhaustedDirectory(t *testing.T) {
	if ResolveSpace.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("rollout=%q", ResolveSpace.OutputRollout)
	}
	caller := &resolveSpaceProjectionCaller{}
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(ResolveSpace))
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--name", "Product", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	code, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted || code != 0 || caller.calls != 2 || stderr.Len() != 0 {
		t.Fatalf("code=%d emitted=%v calls=%d stderr=%q err=%v", code, emitted, caller.calls, stderr.String(), err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope["ok"] != true || envelope["outcome"] != "success" {
		t.Fatalf("envelope=%#v err=%v output=%q", envelope, err, stdout.String())
	}
	if _, exists := envelope["contract_version"]; exists {
		t.Fatalf("removed version marker leaked: %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["resolved"] != false || data["count"] != float64(2) {
		t.Fatalf("data=%#v", envelope["data"])
	}
	candidates, ok := data["candidates"].([]any)
	if !ok || len(candidates) != 2 || candidates[0].(map[string]any)["spaceId"] != "workspace-2" {
		t.Fatalf("candidates=%#v", data["candidates"])
	}
	meta, ok := envelope["meta"].(map[string]any)
	if !ok || meta["count"] != float64(2) {
		t.Fatalf("meta=%#v", envelope["meta"])
	}
	pagination, ok := meta["pagination"].(map[string]any)
	if !ok || pagination["endpoint_exhausted"] != true || pagination["pages"] != float64(2) || pagination["items"] != float64(2) {
		t.Fatalf("pagination=%#v", meta["pagination"])
	}
}

func TestResolveSpaceCollectionRejectsRepeatedWorkspaceID(t *testing.T) {
	caller := &resolveSpaceProjectionCaller{pages: []map[string]any{
		resolveSpacePageFixture(true, "page-2", resolveSpaceRow("workspace-1", "Product")),
		resolveSpacePageFixture(false, "", resolveSpaceRow("workspace-1", "Product Archive")),
	}}
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(ResolveSpace))
	cmd.SetArgs([]string{"--name", "Product"})
	err := cmd.Execute()
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed.StableSubtype != string(apperrors.SubtypeProjectionUnknown) || typed.Retryable {
		t.Fatalf("err=%#v typed=%#v", err, typed)
	}
	if caller.calls != 2 {
		t.Fatalf("calls=%d, want 2", caller.calls)
	}
}

func TestResolveSpaceCollectionRejectsRepeatedCursor(t *testing.T) {
	caller := &resolveSpaceProjectionCaller{pages: []map[string]any{
		resolveSpacePageFixture(true, "page-2", resolveSpaceRow("workspace-1", "Product")),
		resolveSpacePageFixture(true, "page-2", resolveSpaceRow("workspace-2", "Product Archive")),
	}}
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(ResolveSpace))
	cmd.SetArgs([]string{"--name", "Product"})
	err := cmd.Execute()
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed.StableSubtype != string(apperrors.SubtypePaginationInconsistent) || typed.Retryable {
		t.Fatalf("err=%#v typed=%#v", err, typed)
	}
	if caller.calls != 2 {
		t.Fatalf("calls=%d, want 2", caller.calls)
	}
}
