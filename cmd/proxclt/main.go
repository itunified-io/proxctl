// proxclt — Proxmox VM provisioning CLI.
// See docs/ and design doc 024-proxclt-design.md for the full spec.
package main

import "github.com/itunified-io/proxclt/internal/root"

func main() {
	root.Execute()
}
