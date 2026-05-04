package kickstart

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ReadVolumeLabel returns the ISO9660 volume ID (Volume id) of sourceISO using
// xorriso. The label is what Anaconda matches against `inst.stage2=hd:LABEL=`
// and `inst.repo=hd:LABEL=` so the kickstart-only ISO can find the upstream
// full install ISO attached at a second CDROM drive on the VM.
//
// Implementation: parses `xorriso -indev <iso> -toc -report_about WARNING`
// output for the line `Volume id    : <label>`.
func ReadVolumeLabel(sourceISO string) (string, error) {
	if sourceISO == "" {
		return "", fmt.Errorf("ReadVolumeLabel: sourceISO is required")
	}
	if _, err := os.Stat(sourceISO); err != nil {
		return "", fmt.Errorf("source ISO not found: %w", err)
	}
	if _, err := exec.LookPath("xorriso"); err != nil {
		return "", fmt.Errorf("xorriso not found in PATH: %w", err)
	}
	// Use plain `-indev <iso>` (no -toc) — xorriso prints `Volume id : '<label>'`
	// in the standard drive-summary block emitted on indev open. The -toc
	// subcommand suppresses this line on some builds (xorriso 1.5.8).
	cmd := exec.Command("xorriso",
		"-indev", sourceISO,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xorriso -toc failed: %w: %s", err, string(out))
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Match e.g. "Volume id    : 'OL-9-4-0-BaseOS-x86_64'"
		// or "Volume id    : OL-9-4-0-BaseOS-x86_64"
		if strings.HasPrefix(line, "Volume id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			label := strings.TrimSpace(parts[1])
			label = strings.Trim(label, "'\"")
			if label != "" {
				return label, nil
			}
		}
	}
	return "", fmt.Errorf("could not find Volume id in xorriso -toc output")
}

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
