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

package cobracmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestSetOverridePriority(t *testing.T) {
	t.Parallel()

	t.Run("nil cmd is safe", func(t *testing.T) {
		t.Parallel()
		SetOverridePriority(nil, 5) // must not panic
	})

	t.Run("sets priority on cmd with nil annotations", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		SetOverridePriority(cmd, 10)

		if cmd.Annotations == nil {
			t.Fatal("expected Annotations to be initialized")
		}
		if got := cmd.Annotations["dws.override-priority"]; got != "10" {
			t.Fatalf("annotation = %q, want %q", got, "10")
		}
	})

	t.Run("overwrites existing priority", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		SetOverridePriority(cmd, 3)
		SetOverridePriority(cmd, 7)

		if got := cmd.Annotations["dws.override-priority"]; got != "7" {
			t.Fatalf("annotation = %q, want %q", got, "7")
		}
	})
}

func TestOverridePriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  *cobra.Command
		want int
	}{
		{
			name: "nil cmd returns 0",
			cmd:  nil,
			want: 0,
		},
		{
			name: "nil annotations returns 0",
			cmd:  &cobra.Command{Use: "test"},
			want: 0,
		},
		{
			name: "empty annotation returns 0",
			cmd: &cobra.Command{
				Use:         "test",
				Annotations: map[string]string{"dws.override-priority": ""},
			},
			want: 0,
		},
		{
			name: "whitespace-only annotation returns 0",
			cmd: &cobra.Command{
				Use:         "test",
				Annotations: map[string]string{"dws.override-priority": "   "},
			},
			want: 0,
		},
		{
			name: "valid positive value",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				SetOverridePriority(cmd, 42)
				return cmd
			}(),
			want: 42,
		},
		{
			name: "valid negative value",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "test"}
				SetOverridePriority(cmd, -3)
				return cmd
			}(),
			want: -3,
		},
		{
			name: "invalid non-numeric value returns 0",
			cmd: &cobra.Command{
				Use:         "test",
				Annotations: map[string]string{"dws.override-priority": "abc"},
			},
			want: 0,
		},
		{
			name: "annotation key missing returns 0",
			cmd: &cobra.Command{
				Use:         "test",
				Annotations: map[string]string{"other-key": "5"},
			},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := OverridePriority(tc.cmd)
			if got != tc.want {
				t.Fatalf("OverridePriority() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestShouldReplaceLeaf(t *testing.T) {
	t.Parallel()

	t.Run("nil inputs return false", func(t *testing.T) {
		t.Parallel()
		if ShouldReplaceLeaf(nil, nil) {
			t.Fatal("expected false for nil inputs")
		}
		if ShouldReplaceLeaf(&cobra.Command{Use: "a"}, nil) {
			t.Fatal("expected false when src is nil")
		}
		if ShouldReplaceLeaf(nil, &cobra.Command{Use: "a"}) {
			t.Fatal("expected false when dst is nil")
		}
	})

	t.Run("non-leaf commands return false", func(t *testing.T) {
		t.Parallel()
		dst := &cobra.Command{Use: "dst"}
		dst.AddCommand(&cobra.Command{Use: "child"})
		src := &cobra.Command{Use: "src"}

		if ShouldReplaceLeaf(dst, src) {
			t.Fatal("expected false when dst has children")
		}

		dst2 := &cobra.Command{Use: "dst2"}
		src2 := &cobra.Command{Use: "src2"}
		src2.AddCommand(&cobra.Command{Use: "child"})

		if ShouldReplaceLeaf(dst2, src2) {
			t.Fatal("expected false when src has children")
		}
	})

	t.Run("same priority different flag counts", func(t *testing.T) {
		t.Parallel()
		dst := &cobra.Command{Use: "leaf"}
		dst.Flags().String("a", "", "flag a")

		src := &cobra.Command{Use: "leaf"}
		src.Flags().String("a", "", "flag a")
		src.Flags().String("b", "", "flag b")

		if !ShouldReplaceLeaf(dst, src) {
			t.Fatal("expected true when src has more flags at same priority")
		}
	})

	t.Run("same priority same flag counts returns false", func(t *testing.T) {
		t.Parallel()
		dst := &cobra.Command{Use: "leaf"}
		dst.Flags().String("a", "", "flag a")

		src := &cobra.Command{Use: "leaf"}
		src.Flags().String("a", "", "flag a")

		if ShouldReplaceLeaf(dst, src) {
			t.Fatal("expected false when equal flags and priority")
		}
	})

	t.Run("different priorities", func(t *testing.T) {
		t.Parallel()
		dst := &cobra.Command{Use: "leaf"}
		SetOverridePriority(dst, 1)

		src := &cobra.Command{Use: "leaf"}
		SetOverridePriority(src, 5)

		if !ShouldReplaceLeaf(dst, src) {
			t.Fatal("expected true when src has higher priority")
		}

		// Swap: src has lower priority.
		dst2 := &cobra.Command{Use: "leaf"}
		SetOverridePriority(dst2, 10)

		src2 := &cobra.Command{Use: "leaf"}
		SetOverridePriority(src2, 2)

		if ShouldReplaceLeaf(dst2, src2) {
			t.Fatal("expected false when src has lower priority")
		}
	})
}
