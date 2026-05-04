package kickstart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RequiredBootloaderFiles lists the files an OL8/OL9 install ISO must contain
// under /isolinux/ for the legacy ISOBuilder to remaster a kickstart-only ISO.
var RequiredBootloaderFiles = []string{
	"isolinux.bin",
	"ldlinux.c32",
	"vmlinuz",
	"initrd.img",
}

// ExtractBootloader extracts the four files RequiredBootloaderFiles from the
// /isolinux/ directory of sourceISO into destDir using xorriso -osirrox.
// destDir must already exist.
//
// Implementation notes:
//
//   - Oracle Linux 8/9, RHEL 9, and Rocky 9 install ISOs ship the BIOS
//     bootloader at /isolinux/. This is the canonical layout.
//   - We use xorriso `-osirrox on` to read files out of an ISO. The
//     `-extract <iso-path> <host-path>` form copies a directory; if the
//     extraction overshoots and grabs extra files, that's harmless — we
//     only verify the required four are present.
//   - Older RHEL-family install ISOs sometimes name initrd.img differently
//     (e.g. initrd-arm.img); for now we assume the canonical x86_64 layout.
//     Future enhancement: glob + pick.
func ExtractBootloader(sourceISO, destDir string) error {
	if sourceISO == "" {
		return fmt.Errorf("ExtractBootloader: sourceISO is required")
	}
	if destDir == "" {
		return fmt.Errorf("ExtractBootloader: destDir is required")
	}
	if _, err := os.Stat(sourceISO); err != nil {
		return fmt.Errorf("source ISO not found: %w", err)
	}
	if _, err := exec.LookPath("xorriso"); err != nil {
		return fmt.Errorf("xorriso not found in PATH (install xorriso): %w", err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("mkdir destDir: %w", err)
	}

	// xorriso `-extract` copies the ISO source path into the host destination
	// path. We extract /isolinux/ as a directory; xorriso will create destDir
	// contents directly (not a nested isolinux/ subdir) when destDir already
	// exists.
	cmd := exec.Command("xorriso",
		"-osirrox", "on",
		"-indev", sourceISO,
		"-extract", "/isolinux", destDir,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xorriso extract /isolinux failed: %w: %s", err, string(out))
	}

	// Verify all required files are present.
	for _, name := range RequiredBootloaderFiles {
		p := filepath.Join(destDir, name)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("required bootloader file %q missing after extraction (looked in %s): %w", name, destDir, err)
		}
	}
	return nil
}
