package config

// Cluster is the cluster.yaml manifest (§6.2) — multi-VM grouping + shared disks.
type Cluster struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"` // "Cluster"
	Name       string       `yaml:"name" json:"name"`
	Members    []string     `yaml:"members" json:"members"` // VM names
	SharedDisks []SharedDisk `yaml:"shared_disks,omitempty" json:"shared_disks,omitempty"`
}

// SharedDisk describes a disk attached to multiple VMs (e.g. Oracle ASM).
type SharedDisk struct {
	Name    string `yaml:"name" json:"name"`
	Size    string `yaml:"size" json:"size"`
	Storage string `yaml:"storage" json:"storage"`
}
