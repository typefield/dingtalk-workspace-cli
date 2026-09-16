package output

import (
	"github.com/spf13/cobra"
	"testing"
)

func TestResolveFieldsShadowing(t *testing.T) {
	t.Run("global persistent flag propagates", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "dws"}
		rootCmd.PersistentFlags().String("fields", "", "筛选输出字段 (逗号分隔, 如: name,id,status)")

		normalCmd := &cobra.Command{Use: "normal"}
		rootCmd.AddCommand(normalCmd)
		rootCmd.SetArgs([]string{"normal", "--fields", "data,status"})
		rootCmd.Execute()

		if fields := ResolveFields(normalCmd); fields != "data,status" {
			t.Errorf("expected 'data,status' for normal cmd, got %q", fields)
		}
	})

	t.Run("shadowed local flag is ignored", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "dws"}
		rootCmd.PersistentFlags().String("fields", "", "筛选输出字段 (逗号分隔, 如: name,id,status)")

		bizCmd := &cobra.Command{Use: "biz"}
		bizCmd.Flags().String("fields", "", "JSON string array of objects")
		rootCmd.AddCommand(bizCmd)

		rootCmd.SetArgs([]string{"biz", "--fields", "[\"fake\"]"})
		rootCmd.Execute()

		if fields := ResolveFields(bizCmd); fields != "" {
			t.Errorf("expected empty fields for shadowed cmd since it's a business param, got %q", fields)
		}
	})
}
