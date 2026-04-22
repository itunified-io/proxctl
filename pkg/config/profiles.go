package config

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
)

//go:embed profiles/*.yaml
var profilesFS embed.FS

// LoadProfile loads a named profile from the embedded library.
// Phase 2: returns the raw YAML bytes; callers merge selectively.
func LoadProfile(name string) ([]byte, error) {
	path := filepath.Join("profiles", name+".yaml")
	b, err := fs.ReadFile(profilesFS, path)
	if err != nil {
		return nil, fmt.Errorf("profile %q: %w", name, err)
	}
	return b, nil
}

// ListProfiles returns the names of all embedded profiles (without extension).
func ListProfiles() ([]string, error) {
	entries, err := fs.ReadDir(profilesFS, "profiles")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if filepath.Ext(n) == ".yaml" {
			names = append(names, n[:len(n)-5])
		}
	}
	return names, nil
}
