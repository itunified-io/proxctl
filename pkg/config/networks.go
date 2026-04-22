package config

// Networks is the networks.yaml manifest (§6.2).
type Networks struct {
	APIVersion string             `yaml:"apiVersion" json:"apiVersion"`
	Kind       string             `yaml:"kind" json:"kind"` // "Networks"
	Networks   map[string]Network `yaml:"networks" json:"networks"`
}

// Network describes a single named network → bridge mapping.
type Network struct {
	Bridge  string   `yaml:"bridge" json:"bridge"`
	Subnet  string   `yaml:"subnet,omitempty" json:"subnet,omitempty"`
	Gateway string   `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	DNS     []string `yaml:"dns,omitempty" json:"dns,omitempty"`
}
