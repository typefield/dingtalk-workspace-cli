package cli_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app"
)

func TestDebugTeambitionTree(t *testing.T) {
	cmd := app.NewRootCommand()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "teambition" {
			fmt.Println("Found teambition command, Hidden=", sub.Hidden)
			for _, child := range sub.Commands() {
				fmt.Printf("  child: %s Hidden=%v\n", child.Name(), child.Hidden)
				for _, grandchild := range child.Commands() {
					fmt.Printf("    grandchild: %s Hidden=%v\n", grandchild.Name(), grandchild.Hidden)
				}
			}
		}
	}
}

func TestDebugTeambitionExecute(t *testing.T) {

	cmd := app.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-f", "json", "teambition", "raw", "create_project"})

	err := cmd.Execute()
	fmt.Printf("err=%v\nout=%s\n", err, out.String())
}
