// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package launcher

import "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/schemafastpath"

func trySchema(options Options, deps dependencies) (bool, error) {
	// The launcher has no tracker. Default telemetry remains owned by core.
	if !telemetryOptedOut(deps.environ) {
		return false, nil
	}
	prepared, ok := schemafastpath.Prepare(options.Edition, options.SchemaIdentity, schemafastpath.Dependencies{
		Args: deps.args, Environment: deps.environ, Lstat: deps.lstat, OpenCache: deps.openSchemaCache,
	})
	if !ok {
		return false, nil
	}
	return true, prepared.Write(deps.stdout)
}
