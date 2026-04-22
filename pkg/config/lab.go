// Package config defines the env manifest structs for proxclt.
//
// The top-level manifest is called "Env" per design doc §6 (kind: Env).
// Legacy references to "Lab" map to the same struct.
package config

// APIVersionV1 is the current apiVersion for all proxclt manifests.
const APIVersionV1 = "proxclt/v1"

// Env is the root env manifest (env.yaml) referenced by every proxclt invocation.
// See design doc §6.2 for the full schema.
type Env struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"` // "Env"
	Metadata   EnvMetadata  `yaml:"metadata" json:"metadata"`
	Layers     EnvLayers    `yaml:"layers" json:"layers"`
	Defaults   EnvDefaults  `yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

// EnvMetadata holds descriptive fields.
type EnvMetadata struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// EnvLayers enumerates the $ref-composed child documents.
// Phase 1 uses string paths; Phase 2 will upgrade to a polymorphic $ref resolver.
type EnvLayers struct {
	Hypervisor string   `yaml:"hypervisor,omitempty" json:"hypervisor,omitempty"`
	Networks   string   `yaml:"networks,omitempty" json:"networks,omitempty"`
	Storage    string   `yaml:"storage,omitempty" json:"storage,omitempty"`
	Cluster    string   `yaml:"cluster,omitempty" json:"cluster,omitempty"`
	VMs        []string `yaml:"vms,omitempty" json:"vms,omitempty"`
}

// EnvDefaults are env-wide defaults overridable per-VM.
type EnvDefaults struct {
	Distro   string `yaml:"distro,omitempty" json:"distro,omitempty"`
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
	Keyboard string `yaml:"keyboard,omitempty" json:"keyboard,omitempty"`
	Lang     string `yaml:"lang,omitempty" json:"lang,omitempty"`
}

// Lab is a legacy alias for Env kept during the scaffold phase.
type Lab = Env

// Validate performs a basic structural check. Phase 1: only verifies
// apiVersion + kind; full JSON Schema validation lands in Phase 2.
func (e *Env) Validate() error {
	if e == nil {
		return errEmpty
	}
	if e.APIVersion == "" {
		return errMissingAPIVersion
	}
	if e.Kind == "" {
		return errMissingKind
	}
	return nil
}
