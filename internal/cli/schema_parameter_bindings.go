// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

const schemaParameterBindingsVersion = 2

const runtimeSchemaFlagBindingPropertyAnnotation = "dws.schema.binding.property"

//go:embed schema_parameter_bindings.json
var embeddedSchemaParameterBindingsJSON []byte

type schemaParameterBindingSnapshot struct {
	Version                int                          `json:"version"`
	HistoricalBindingCount int                          `json:"historical_binding_count"`
	Migrations             map[string]string            `json:"migrations"`
	Excluded               map[string]string            `json:"excluded"`
	Added                  map[string]string            `json:"added"`
	Bindings               map[string]map[string]string `json:"bindings"`
}

var runtimeSchemaParameterBindingsLazy struct {
	once     sync.Once
	snapshot schemaParameterBindingSnapshot
}

var runtimeSchemaParameterBindingsLazyLoadCount atomic.Uint64

func loadSchemaParameterBindings() schemaParameterBindingSnapshot {
	var snapshot schemaParameterBindingSnapshot
	if err := json.Unmarshal(embeddedSchemaParameterBindingsJSON, &snapshot); err != nil ||
		snapshot.Version != schemaParameterBindingsVersion {
		return schemaParameterBindingSnapshot{}
	}
	return snapshot
}

func runtimeSchemaParameterBindingData() schemaParameterBindingSnapshot {
	runtimeSchemaParameterBindingsLazy.once.Do(func() {
		runtimeSchemaParameterBindingsLazyLoadCount.Add(1)
		runtimeSchemaParameterBindingsLazy.snapshot = loadSchemaParameterBindings()
	})
	return runtimeSchemaParameterBindingsLazy.snapshot
}

func applyRuntimeSchemaParameterBindings(cmd *cobra.Command, canonical string) {
	applyRuntimeSchemaParameterBindingsFrom(cmd, canonical, runtimeSchemaParameterBindingData().Bindings)
}

func applyRuntimeSchemaParameterBindingsFrom(cmd *cobra.Command, canonical string, bindings map[string]map[string]string) {
	for flagName, propertyName := range bindings[strings.TrimSpace(canonical)] {
		if flag := runtimeCommandFlag(cmd, flagName); flag != nil {
			setFlagAnnotation(flag, runtimeSchemaFlagBindingPropertyAnnotation, strings.TrimSpace(propertyName))
		}
	}
}

// EmbeddedSchemaParameterBindings returns a defensive copy of the reviewed,
// active public flag-to-interface bindings used by Catalog generation.
func EmbeddedSchemaParameterBindings() map[string]map[string]string {
	source := runtimeSchemaParameterBindingData().Bindings
	bindings := make(map[string]map[string]string, len(source))
	for canonical, parameters := range source {
		bindings[canonical] = make(map[string]string, len(parameters))
		for flagName, propertyName := range parameters {
			bindings[canonical][flagName] = propertyName
		}
	}
	return bindings
}
