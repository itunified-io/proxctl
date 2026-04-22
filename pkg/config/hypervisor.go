package config

// Hypervisor is the hypervisor.yaml manifest (§6.2).
type Hypervisor struct {
	APIVersion     string   `yaml:"apiVersion" json:"apiVersion"`
	Kind           string   `yaml:"kind" json:"kind"` // "Hypervisor"
	Context        string   `yaml:"context" json:"context"`
	Nodes          []string `yaml:"nodes" json:"nodes"`
	DefaultStorage string   `yaml:"default_storage,omitempty" json:"default_storage,omitempty"`
	DefaultBridge  string   `yaml:"default_bridge,omitempty" json:"default_bridge,omitempty"`
}
