package config

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validateOnce sync.Once
	validate     *validator.Validate
)

// Validator returns the process-wide validator instance, lazily initialized.
func Validator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New(validator.WithRequiredStructEnabled())
	})
	return validate
}

// Validate runs struct-tag validation + cross-field rules on env.
func Validate(env *Env) error {
	if env == nil {
		return errors.New("validate: nil env")
	}
	v := Validator()
	if err := v.Struct(env); err != nil {
		return fmt.Errorf("struct validation: %w", err)
	}
	// Validate resolved children.
	if h := env.Spec.Hypervisor.Resolved(); h != nil {
		if err := v.Struct(h); err != nil {
			return fmt.Errorf("hypervisor: %w", err)
		}
	}
	if n := env.Spec.Networks.Resolved(); n != nil {
		if err := v.Struct(n); err != nil {
			return fmt.Errorf("networks: %w", err)
		}
	}
	if sc := env.Spec.StorageClasses.Resolved(); sc != nil {
		if err := v.Struct(sc); err != nil {
			return fmt.Errorf("storage_classes: %w", err)
		}
	}
	if env.Spec.Cluster != nil {
		if c := env.Spec.Cluster.Resolved(); c != nil {
			if err := v.Struct(c); err != nil {
				return fmt.Errorf("cluster: %w", err)
			}
		}
	}
	if env.Spec.Linux != nil {
		if l := env.Spec.Linux.Resolved(); l != nil {
			if err := v.Struct(l); err != nil {
				return fmt.Errorf("linux: %w", err)
			}
		}
	}
	for i, d := range env.Spec.Databases {
		if r := d.Resolved(); r != nil {
			if err := v.Struct(r); err != nil {
				return fmt.Errorf("databases[%d]: %w", i, err)
			}
		}
	}

	// Cross-field rules.
	var errs []error
	if err := validateUniqueness(env); err != nil {
		errs = append(errs, err)
	}
	if err := validateRACInvariants(env); err != nil {
		errs = append(errs, err)
	}
	if err := validateNICNetworkRefs(env); err != nil {
		errs = append(errs, err)
	}
	if err := validateDiskStorageRefs(env); err != nil {
		errs = append(errs, err)
	}
	if err := validateDiskTagRefs(env); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// validateUniqueness verifies VMIDs, hostnames, MACs, and IPs (per network) are unique.
func validateUniqueness(env *Env) error {
	h := env.Spec.Hypervisor.Resolved()
	if h == nil {
		return nil
	}
	vmIDs := map[int]string{}
	macs := map[string]string{}
	ipByZone := map[string]map[string]string{} // zone → ip → owner
	hostnames := map[string]bool{}

	for name, node := range h.Nodes {
		if hostnames[name] {
			return fmt.Errorf("duplicate hostname: %q", name)
		}
		hostnames[name] = true

		if prev, ok := vmIDs[node.Proxmox.VMID]; ok {
			return fmt.Errorf("duplicate vm_id %d: %q and %q", node.Proxmox.VMID, prev, name)
		}
		vmIDs[node.Proxmox.VMID] = name

		for _, nic := range node.NICs {
			if nic.MAC != "" && nic.MAC != "auto" {
				if prev, ok := macs[nic.MAC]; ok {
					return fmt.Errorf("duplicate MAC %s: %q and %q", nic.MAC, prev, name)
				}
				macs[nic.MAC] = name
			}
			if nic.Network != "" && nic.IPv4 != nil && nic.IPv4.Address != "" {
				zoneMap, ok := ipByZone[nic.Network]
				if !ok {
					zoneMap = map[string]string{}
					ipByZone[nic.Network] = zoneMap
				}
				addr := strings.SplitN(nic.IPv4.Address, "/", 2)[0]
				if prev, ok := zoneMap[addr]; ok {
					return fmt.Errorf("duplicate IP %s in network %q: %q and %q", addr, nic.Network, prev, name)
				}
				zoneMap[addr] = name
			}
		}
	}
	return nil
}

// validateRACInvariants checks Oracle RAC cluster constraints.
func validateRACInvariants(env *Env) error {
	if env.Spec.Cluster == nil {
		return nil
	}
	c := env.Spec.Cluster.Resolved()
	if c == nil || c.Type != "oracle-rac" {
		return nil
	}
	if len(c.ScanIPs) < 3 {
		return fmt.Errorf("oracle-rac: scan_ips must have ≥3 IPs, got %d", len(c.ScanIPs))
	}
	h := env.Spec.Hypervisor.Resolved()
	if h == nil {
		return nil
	}
	for name, node := range h.Nodes {
		hasPublic, hasPrivate := false, false
		for _, nic := range node.NICs {
			switch nic.Usage {
			case "public":
				hasPublic = true
			case "private":
				hasPrivate = true
			}
		}
		if !hasPublic {
			return fmt.Errorf("oracle-rac: node %q missing public NIC", name)
		}
		if !hasPrivate {
			return fmt.Errorf("oracle-rac: node %q missing private NIC", name)
		}
	}
	return nil
}

// validateNICNetworkRefs ensures every NIC.Network references a known zone.
func validateNICNetworkRefs(env *Env) error {
	h := env.Spec.Hypervisor.Resolved()
	n := env.Spec.Networks.Resolved()
	if h == nil || n == nil {
		return nil
	}
	for nodeName, node := range h.Nodes {
		for i, nic := range node.NICs {
			if nic.Network == "" {
				continue
			}
			if _, ok := n.Zones[nic.Network]; !ok {
				return fmt.Errorf("node %q nics[%d]: network %q not defined", nodeName, i, nic.Network)
			}
		}
	}
	return nil
}

// validateDiskStorageRefs ensures every Disk.StorageClass references a known class.
func validateDiskStorageRefs(env *Env) error {
	h := env.Spec.Hypervisor.Resolved()
	sc := env.Spec.StorageClasses.Resolved()
	if h == nil || sc == nil {
		return nil
	}
	for nodeName, node := range h.Nodes {
		for i, d := range node.Disks {
			if d.StorageClass == "" {
				continue
			}
			if _, ok := sc.Classes[d.StorageClass]; !ok {
				return fmt.Errorf("node %q disks[%d]: storage_class %q not defined", nodeName, i, d.StorageClass)
			}
		}
	}
	return nil
}

// validateDiskTagRefs ensures every disk tag referenced from linux.disk_layout.additional
// exists somewhere in the hypervisor disk set.
func validateDiskTagRefs(env *Env) error {
	if env.Spec.Linux == nil {
		return nil
	}
	l := env.Spec.Linux.Resolved()
	if l == nil {
		return nil
	}
	layout, _ := l.Raw["disk_layout"].(map[string]any)
	if layout == nil {
		return nil
	}
	additional, _ := layout["additional"].([]any)
	if additional == nil {
		return nil
	}
	hyp := env.Spec.Hypervisor.Resolved()
	if hyp == nil {
		return nil
	}
	tags := map[string]bool{}
	for _, node := range hyp.Nodes {
		for _, d := range node.Disks {
			if d.Tag != "" {
				tags[d.Tag] = true
			}
		}
	}
	for i, entry := range additional {
		m, _ := entry.(map[string]any)
		if m == nil {
			continue
		}
		tag, _ := m["tag"].(string)
		if tag == "" {
			continue
		}
		if !tags[tag] {
			return fmt.Errorf("linux.disk_layout.additional[%d]: tag %q not present on any hypervisor disk", i, tag)
		}
	}
	return nil
}
