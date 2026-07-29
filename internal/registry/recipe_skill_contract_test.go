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

package registry

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type recipeRegistryDocument struct {
	Recipes []recipeRegistryEntry `yaml:"recipes"`
}

type recipeRegistryEntry struct {
	Name  string   `yaml:"name"`
	Steps []string `yaml:"steps"`
}

var recipeCommandPattern = regexp.MustCompile("`(dws [^`]+)`")

func TestLarkAlignedRecipeSkillsMatchRegistry(t *testing.T) {
	var registry recipeRegistryDocument
	if err := yaml.Unmarshal(RecipesYAML(), &registry); err != nil {
		t.Fatalf("parse recipes registry: %v", err)
	}

	entries := make(map[string]recipeRegistryEntry, len(registry.Recipes))
	for _, recipe := range registry.Recipes {
		entries[recipe.Name] = recipe
	}

	for _, name := range []string{"generate-meeting-summary", "generate-standup-report"} {
		t.Run(name, func(t *testing.T) {
			entry, ok := entries[name]
			if !ok {
				t.Fatalf("recipe %q missing from embedded registry", name)
			}

			skillPath := filepath.Join("..", "..", "skills", "multi", "recipe-"+name, "SKILL.md")
			data, err := os.ReadFile(skillPath)
			if err != nil {
				t.Fatalf("read recipe skill %s: %v", skillPath, err)
			}
			skill := string(data)
			for _, required := range []string{
				"name: recipe-" + name,
				`category: "recipe"`,
				"## Steps",
			} {
				if !strings.Contains(skill, required) {
					t.Errorf("recipe skill missing %q", required)
				}
			}

			var registryCommands []string
			for _, step := range entry.Steps {
				match := recipeCommandPattern.FindStringSubmatch(step)
				if len(match) != 2 {
					t.Fatalf("registry step does not contain exactly one dws command: %q", step)
				}
				registryCommands = append(registryCommands, match[1])
			}

			var skillCommands []string
			for _, match := range recipeCommandPattern.FindAllStringSubmatch(skill, -1) {
				skillCommands = append(skillCommands, match[1])
			}
			if !reflect.DeepEqual(skillCommands, registryCommands) {
				t.Fatalf("SKILL.md commands differ from registry\nskill:    %q\nregistry: %q", skillCommands, registryCommands)
			}
		})
	}
}
