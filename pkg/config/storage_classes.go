package config

// StorageClasses is the storage-classes.yaml manifest (§6.2).
type StorageClasses struct {
	APIVersion string                  `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                  `yaml:"kind" json:"kind"` // "StorageClasses"
	Classes    map[string]StorageClass `yaml:"classes" json:"classes"`
}

// StorageClass names a PVE storage target + format policy.
type StorageClass struct {
	PVEStorage string `yaml:"pve_storage" json:"pve_storage"`
	Format     string `yaml:"format,omitempty" json:"format,omitempty"`
}
