// Package recipes contains the build recipe registry.
package recipes

import (
	"fmt"

	"github.com/mah3sec/forgeguardian/internal/core"
)

var registered = map[string]core.BuildRecipe{}

// Register adds a recipe to the global registry.
func Register(r core.BuildRecipe) {
	registered[r.Ecosystem()] = r
}

// Get returns the recipe for the given ecosystem, or an error if not found.
func Get(ecosystem string) (core.BuildRecipe, error) {
	r, ok := registered[ecosystem]
	if !ok {
		return nil, fmt.Errorf("recipes: no recipe registered for ecosystem %q", ecosystem)
	}
	return r, nil
}

// All returns the names of all registered ecosystems, sorted.
func All() []string {
	names := make([]string, 0, len(registered))
	for k := range registered {
		names = append(names, k)
	}
	return names
}

// AllRecipes returns the full recipe map.
func AllRecipes() map[string]core.BuildRecipe { return registered }
