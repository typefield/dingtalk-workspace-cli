// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package buildversion

import "testing"

func TestCrossPlatformCoverageFormatIncludesMetadataUnlessUnknown(t *testing.T) {
	if got := Format("1.0.0", "unknown", "unknown"); got != "1.0.0" {
		t.Fatalf("unknown metadata = %q", got)
	}
	if got := Format("1.0.0", "abc", "unknown"); got != "1.0.0 (abc, unknown)" {
		t.Fatalf("commit metadata = %q", got)
	}
	if got := Format("1.0.0", "unknown", "now"); got != "1.0.0 (unknown, now)" {
		t.Fatalf("time metadata = %q", got)
	}
}
