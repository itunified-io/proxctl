package config

import (
	"strings"
	"testing"
)

func baseEnv() *Env {
	return &Env{
		Version: "1",
		Kind:    "Env",
		Metadata: EnvMetadata{Name: "test"},
		Spec: EnvSpec{
			Hypervisor: Ref[Hypervisor]{Value: &Hypervisor{
				Kind: "Hypervisor",
				Nodes: map[string]Node{
					"n1": {
						Proxmox: ProxmoxRef{NodeName: "pve", VMID: 100},
						IPs:     map[string]string{"public": "10.0.0.1"},
						NICs: []NIC{
							{NameField: "net0", Usage: "public", Network: "public", IPv4: &IPv4Config{Address: "10.0.0.1/24"}},
						},
						Disks: []Disk{{ID: 0, Size: "10G", StorageClass: "local"}},
					},
				},
			}},
			Networks: Ref[Networks]{Value: &Networks{
				Kind:  "Networks",
				Zones: map[string]NetworkZone{"public": {CIDR: "10.0.0.0/24"}},
			}},
			StorageClasses: Ref[StorageClasses]{Value: &StorageClasses{
				Kind:    "StorageClasses",
				Classes: map[string]StorageClass{"local": {Backend: "lvm"}},
			}},
		},
	}
}

func TestValidate_Happy(t *testing.T) {
	if err := Validate(baseEnv()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidate_DuplicateVMID(t *testing.T) {
	env := baseEnv()
	h := env.Spec.Hypervisor.Value
	h.Nodes["n2"] = Node{
		Proxmox: ProxmoxRef{NodeName: "pve", VMID: 100},
		IPs:     map[string]string{"public": "10.0.0.2"},
		NICs:    []NIC{{NameField: "net0", Usage: "public", Network: "public", IPv4: &IPv4Config{Address: "10.0.0.2/24"}}},
	}
	err := Validate(env)
	if err == nil || !strings.Contains(err.Error(), "vm_id") {
		t.Fatalf("want vm_id dup error, got %v", err)
	}
}

func TestValidate_UnknownNetworkRef(t *testing.T) {
	env := baseEnv()
	node := env.Spec.Hypervisor.Value.Nodes["n1"]
	node.NICs[0].Network = "nope"
	env.Spec.Hypervisor.Value.Nodes["n1"] = node
	err := Validate(env)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want network ref error, got %v", err)
	}
}

func TestValidate_UnknownStorageClass(t *testing.T) {
	env := baseEnv()
	node := env.Spec.Hypervisor.Value.Nodes["n1"]
	node.Disks[0].StorageClass = "nope"
	env.Spec.Hypervisor.Value.Nodes["n1"] = node
	err := Validate(env)
	if err == nil || !strings.Contains(err.Error(), "storage_class") {
		t.Fatalf("want storage_class error, got %v", err)
	}
}

func TestValidate_RAC_MissingSCAN(t *testing.T) {
	env := baseEnv()
	env.Spec.Cluster = &Ref[Cluster]{Value: &Cluster{
		Kind: "Cluster",
		Type: "oracle-rac",
		ScanIPs: []string{"10.0.0.10", "10.0.0.11"}, // only 2
	}}
	err := Validate(env)
	if err == nil || !strings.Contains(err.Error(), "scan_ips") {
		t.Fatalf("want scan_ips error, got %v", err)
	}
}

func TestValidate_RAC_MissingPrivateNIC(t *testing.T) {
	env := baseEnv()
	env.Spec.Cluster = &Ref[Cluster]{Value: &Cluster{
		Kind:    "Cluster",
		Type:    "oracle-rac",
		ScanIPs: []string{"10.0.0.10", "10.0.0.11", "10.0.0.12"},
	}}
	// node only has public NIC → missing private
	err := Validate(env)
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("want private NIC error, got %v", err)
	}
}

func TestValidate_DuplicateMAC(t *testing.T) {
	env := baseEnv()
	env.Spec.Hypervisor.Value.Nodes["n1"].NICs[0].MAC = "aa:bb:cc:dd:ee:ff"
	env.Spec.Hypervisor.Value.Nodes["n2"] = Node{
		Proxmox: ProxmoxRef{NodeName: "pve", VMID: 101},
		IPs:     map[string]string{"public": "10.0.0.2"},
		NICs: []NIC{
			{NameField: "net0", Usage: "public", Network: "public", MAC: "aa:bb:cc:dd:ee:ff",
				IPv4: &IPv4Config{Address: "10.0.0.2/24"}},
		},
	}
	err := Validate(env)
	if err == nil || !strings.Contains(err.Error(), "MAC") {
		t.Fatalf("want MAC dup error, got %v", err)
	}
}

func TestValidate_DiskTagCrossRef(t *testing.T) {
	env := baseEnv()
	env.Spec.Hypervisor.Value.Nodes["n1"].Disks[0].Tag = "u01"
	env.Spec.Linux = &Ref[Linux]{Value: &Linux{
		Kind: "Linux",
		Raw: map[string]any{
			"disk_layout": map[string]any{
				"additional": []any{
					map[string]any{"tag": "u01"},
				},
			},
		},
	}}
	if err := Validate(env); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// Now break it.
	env.Spec.Linux.Value.Raw["disk_layout"] = map[string]any{
		"additional": []any{map[string]any{"tag": "missing"}},
	}
	err := Validate(env)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("want missing tag error, got %v", err)
	}
}
