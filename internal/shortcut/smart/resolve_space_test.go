package smart

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
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
}

func TestResolveSpaceCandidateProjectionPreservesAmbiguity(t *testing.T) {
	data := map[string]any{
		"result": map[string]any{
			"wikiSpaces": []any{
				map[string]any{"spaceId": "s1", "name": "产品文档"},
				map[string]any{"space_id": "s2", "spaceName": "产品文档-归档"},
			},
		},
	}
	items := resolveSpaceItems(data)
	if len(items) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(items))
	}
	candidates := make([]map[string]any, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, map[string]any{
			"spaceId": resolveSpaceID(item),
			"name":    resolveSpaceName(item),
		})
	}
	if candidates[0]["spaceId"] != "s1" || candidates[1]["spaceId"] != "s2" {
		t.Fatalf("candidate IDs were not projected: %#v", candidates)
	}
	if candidates[0]["name"] != "产品文档" || candidates[1]["name"] != "产品文档-归档" {
		t.Fatalf("candidate names were not projected: %#v", candidates)
	}
}
