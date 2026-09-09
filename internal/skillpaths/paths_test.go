// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package skillpaths

import "testing"

func TestCrossPlatformCoverageAgentHomesReturnsDetachedCopy(t *testing.T) {
	homes := AgentHomes()
	if len(homes) == 0 {
		t.Fatal("AgentHomes is empty")
	}
	homes[0] = "mutated"
	again := AgentHomes()
	if again[0] == "mutated" {
		t.Fatal("AgentHomes aliased the production slice")
	}
}
