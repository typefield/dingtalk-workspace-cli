package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicRepositoryAssetsExist(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Abs(repo root) error = %v", err)
	}

	for _, rel := range []string{
		".github/workflows/ci.yml",
		".github/PULL_REQUEST_TEMPLATE.md",
		".env.example",
		"docs/architecture.md",
		"scripts/policy/open-source-audit.sh",
	} {
		full := filepath.Join(root, rel)
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("Stat(%s) error = %v", full, err)
		}
	}
}

func TestOpenSourceAuditScriptPasses(t *testing.T) {
	t.Parallel()

	scriptPath, err := filepath.Abs(filepath.Join("..", "..", "scripts", "policy", "check-open-source-assets.sh"))
	if err != nil {
		t.Fatalf("Abs(check-open-source-assets.sh) error = %v", err)
	}

	cmd := exec.Command("sh", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check-open-source-assets.sh error = %v\noutput:\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "open-source audit: ok") {
		t.Fatalf("audit output missing success marker:\n%s", string(output))
	}
}

func TestGeneratedDriftCheckPassesForCleanCheckout(t *testing.T) {
	t.Parallel()

	scriptPath, err := filepath.Abs(filepath.Join("..", "..", "scripts", "policy", "check-generated-drift.sh"))
	if err != nil {
		t.Fatalf("Abs(check-generated-drift.sh) error = %v", err)
	}

	cmd := exec.Command("sh", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check-generated-drift.sh error = %v\noutput:\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "generated drift check: ok") {
		t.Fatalf("drift output missing success marker:\n%s", string(output))
	}
}
