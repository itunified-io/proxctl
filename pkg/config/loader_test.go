package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_Happy(t *testing.T) {
	env, err := Load("testdata/env.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if env.Metadata.Name != "test-env" {
		t.Errorf("name: %q", env.Metadata.Name)
	}
	if h := env.Spec.Hypervisor.Resolved(); h == nil {
		t.Fatal("hypervisor not resolved")
	} else if len(h.Nodes) != 2 {
		t.Errorf("want 2 nodes, got %d", len(h.Nodes))
	}
	if n := env.Spec.Networks.Resolved(); n == nil {
		t.Fatal("networks not resolved")
	} else if _, ok := n.Zones["public"]; !ok {
		t.Error("missing public zone")
	}
}

func TestLoad_InlineOnly(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "env.yaml")
	content := `version: "1"
kind: Env
metadata: {name: inline}
spec:
  hypervisor:
    kind: Hypervisor
    nodes:
      n1:
        proxmox: {node_name: pve, vm_id: 100}
        ips: {public: 10.0.0.1}
  networks:
    kind: Networks
    public: {cidr: 10.0.0.0/24}
  storage_classes:
    kind: StorageClasses
    local: {backend: lvm}
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	env, err := Load(p)
	if err != nil {
		t.Fatalf("Load inline: %v", err)
	}
	if env.Spec.Hypervisor.Resolved().Nodes["n1"].Proxmox.VMID != 100 {
		t.Errorf("inline not loaded")
	}
}

func TestLoad_ProfileExtends(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "env.yaml")
	content := `version: "1"
kind: Env
extends: pg-single
metadata: {name: pg-child}
spec:
  hypervisor:
    kind: Hypervisor
    nodes:
      n1:
        proxmox: {node_name: pve, vm_id: 100}
        ips: {public: 10.0.0.1}
  networks:
    kind: Networks
    public: {cidr: 10.0.0.0/24}
  storage_classes:
    kind: StorageClasses
    local: {backend: lvm}
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// SkipValidate because cluster inherits from profile but has no $ref here.
	_, err := LoadWithOptions(p, LoadOptions{SkipValidate: true})
	if err != nil {
		t.Fatalf("extends: %v", err)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestLoad_BadRef(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "env.yaml")
	content := `version: "1"
kind: Env
metadata: {name: badref}
spec:
  hypervisor: {$ref: ./missing.yaml}
  networks:
    kind: Networks
    public: {cidr: 10.0.0.0/24}
  storage_classes:
    kind: StorageClasses
    local: {backend: lvm}
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "hypervisor") {
		t.Fatalf("want hypervisor ref error, got %v", err)
	}
}

func TestLoadProfiles(t *testing.T) {
	names, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 {
		t.Errorf("want ≥2 profiles, got %v", names)
	}
	for _, n := range names {
		if _, err := LoadProfile(n); err != nil {
			t.Errorf("profile %s: %v", n, err)
		}
	}
}
