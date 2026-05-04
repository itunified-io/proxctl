package kickstart

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExtractBootloader_ValidationErrors covers the synchronous input checks
// that don't require xorriso to be on PATH.
func TestExtractBootloader_ValidationErrors(t *testing.T) {
	if err := ExtractBootloader("", "/tmp/x"); err == nil {
		t.Fatalf("expected error for empty sourceISO")
	}
	if err := ExtractBootloader("/some/iso", ""); err == nil {
		t.Fatalf("expected error for empty destDir")
	}
	if err := ExtractBootloader("/nonexistent/path/to/missing.iso", t.TempDir()); err == nil {
		t.Fatalf("expected error for missing source ISO")
	}
}

// TestExtractBootloader_HappyPath builds a tiny test ISO with mock bootloader
// files under /isolinux/ and verifies ExtractBootloader writes all four files.
// Skipped when xorriso isn't installed (CI environment dependent).
func TestExtractBootloader_HappyPath(t *testing.T) {
	if _, err := exec.LookPath("xorriso"); err != nil {
		t.Skip("xorriso not in PATH; skipping ISO extraction test")
	}

	tmp := t.TempDir()

	// Stage a tree with /isolinux/<files>.
	stage := filepath.Join(tmp, "stage")
	isolinuxDir := filepath.Join(stage, "isolinux")
	if err := os.MkdirAll(isolinuxDir, 0o755); err != nil {
		t.Fatalf("mkdir stage: %v", err)
	}
	for _, name := range RequiredBootloaderFiles {
		// Use distinct content per file so we can sanity-check the extraction
		// preserved file boundaries.
		content := []byte("mock-" + name)
		if err := os.WriteFile(filepath.Join(isolinuxDir, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Build a minimal ISO. We don't need it to be bootable — only readable
	// by xorriso -osirrox.
	isoPath := filepath.Join(tmp, "test.iso")
	cmd := exec.Command("xorriso", "-as", "mkisofs",
		"-o", isoPath,
		"-J", "-r",
		"-V", "TEST",
		stage,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build test ISO: %v: %s", err, string(out))
	}

	// Extract.
	dest := filepath.Join(tmp, "out")
	if err := ExtractBootloader(isoPath, dest); err != nil {
		t.Fatalf("ExtractBootloader: %v", err)
	}

	// Verify each file is present and non-empty.
	for _, name := range RequiredBootloaderFiles {
		p := filepath.Join(dest, name)
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("missing extracted file %s: %v", name, err)
		}
		if fi.Size() == 0 {
			t.Fatalf("extracted file %s is empty", name)
		}
	}
}

// TestExtractBootloader_MissingFile builds an ISO with only some of the
// required files and asserts the post-extract verification errors.
func TestExtractBootloader_MissingFile(t *testing.T) {
	if _, err := exec.LookPath("xorriso"); err != nil {
		t.Skip("xorriso not in PATH; skipping ISO extraction test")
	}

	tmp := t.TempDir()
	stage := filepath.Join(tmp, "stage")
	isolinuxDir := filepath.Join(stage, "isolinux")
	if err := os.MkdirAll(isolinuxDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Only stage isolinux.bin — vmlinuz, initrd.img, ldlinux.c32 missing.
	if err := os.WriteFile(filepath.Join(isolinuxDir, "isolinux.bin"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	isoPath := filepath.Join(tmp, "test.iso")
	cmd := exec.Command("xorriso", "-as", "mkisofs",
		"-o", isoPath,
		"-J", "-r",
		"-V", "TEST",
		stage,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build test ISO: %v: %s", err, string(out))
	}

	dest := filepath.Join(tmp, "out")
	if err := ExtractBootloader(isoPath, dest); err == nil {
		t.Fatalf("expected error for ISO missing required bootloader files")
	}
}
