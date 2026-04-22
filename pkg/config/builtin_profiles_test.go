package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestBuiltinProfiles_HasThree asserts the shipped profile library contains
// the three Phase 5 baselines.
func TestBuiltinProfiles_HasThree(t *testing.T) {
	names := BuiltinProfiles()
	want := map[string]bool{"oracle-rac-2node": false, "pg-single": false, "host-only": false}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("profile %q not found in BuiltinProfiles()=%v", n, names)
		}
	}
}

// TestLoadBuiltinProfile_Parses asserts every shipped profile is valid YAML
// and parses into the Env schema.
func TestLoadBuiltinProfile_Parses(t *testing.T) {
	for _, name := range BuiltinProfiles() {
		b, err := LoadBuiltinProfile(name)
		if err != nil {
			t.Errorf("LoadBuiltinProfile(%q): %v", name, err)
			continue
		}
		if len(b) == 0 {
			t.Errorf("profile %q is empty", name)
			continue
		}
		var env Env
		if err := yaml.Unmarshal(b, &env); err != nil {
			t.Errorf("profile %q failed to parse: %v", name, err)
		}
		if env.Kind != "Env" {
			t.Errorf("profile %q: want kind=Env got %q", name, env.Kind)
		}
	}
}

// TestLoadBuiltinProfile_NotFound covers the error path.
func TestLoadBuiltinProfile_NotFound(t *testing.T) {
	if _, err := LoadBuiltinProfile("no-such-profile"); err == nil {
		t.Error("want error for missing profile")
	}
}
