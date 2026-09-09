// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package localename

import "testing"

func TestCrossPlatformCoverageResolveChineseAndEnglishLocales(t *testing.T) {
	if got := Resolve(" zh_CN.UTF-8 "); got != "zh" {
		t.Fatalf("chinese = %q", got)
	}
	if got := Resolve("EN-us"); got != "en" {
		t.Fatalf("english = %q", got)
	}
	if got := Resolve(""); got != "en" {
		t.Fatalf("blank = %q", got)
	}
}
