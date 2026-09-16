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

package registry

import "embed"

var (
	//go:embed personas.yaml
	personasYAML []byte
	//go:embed recipes.yaml
	recipesYAML []byte
	//go:embed personas.yaml recipes.yaml
	_ embed.FS
)

func PersonasYAML() []byte {
	return append([]byte(nil), personasYAML...)
}

func RecipesYAML() []byte {
	return append([]byte(nil), recipesYAML...)
}
