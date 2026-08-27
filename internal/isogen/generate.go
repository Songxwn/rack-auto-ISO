package isogen

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Asset names expected under AssetsDir (populated by CI).
const (
	AssetIPXELKRN = "ipxe.lkrn"
	AssetIPXEEFI  = "ipxe.efi"
	AssetIsolinux = "isolinux.bin"
	AssetLDLinux  = "ldlinux.c32"
)

// AssetsDir is where optional boot assets live next to the binary or under data.
var AssetsDir string

func ResolveAssetsDir(candidates ...string) string {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if hasAssets(c) {
			return c
		}
	}
	return ""
}

func hasAssets(dir string) bool {
	need := []string{AssetIPXELKRN, AssetIPXEEFI}
	for _, n := range need {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			return false
		}
	}
	return true
}

// GenerateISO writes a BIOS+UEFI hybrid-ish ISO with custom embed.ipxe.
// Requires xorriso in PATH and boot assets (ipxe.lkrn, ipxe.efi; isolinux optional for BIOS).
func GenerateISO(embedScript string, assetsDir, outPath string) error {
	if assetsDir == "" || !hasAssets(assetsDir) {
		return fmt.Errorf("iPXE boot assets not found (need %s and %s); download a Release that includes assets/ or set IPXE_ASSETS", AssetIPXELKRN, AssetIPXEEFI)
	}
	xorriso, err := exec.LookPath("xorriso")
	if err != nil {
		return fmt.Errorf("xorriso not found in PATH (required to build ISO): %w", err)
	}

	tmp, err := os.MkdirTemp("", "ipxe-iso-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := os.WriteFile(filepath.Join(tmp, "embed.ipxe"), []byte(embedScript), 0o644); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(assetsDir, AssetIPXELKRN), filepath.Join(tmp, AssetIPXELKRN)); err != nil {
		return err
	}
	efiDir := filepath.Join(tmp, "EFI", "BOOT")
	if err := os.MkdirAll(efiDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(assetsDir, AssetIPXEEFI), filepath.Join(efiDir, "BOOTX64.EFI")); err != nil {
		return err
	}

	// BIOS boot via isolinux if available
	isolinux := filepath.Join(assetsDir, AssetIsolinux)
	ldlinux := filepath.Join(assetsDir, AssetLDLinux)
	bios := false
	if fileExists(isolinux) && fileExists(ldlinux) {
		isoDir := filepath.Join(tmp, "isolinux")
		if err := os.MkdirAll(isoDir, 0o755); err != nil {
			return err
		}
		if err := copyFile(isolinux, filepath.Join(isoDir, AssetIsolinux)); err != nil {
			return err
		}
		if err := copyFile(ldlinux, filepath.Join(isoDir, AssetLDLinux)); err != nil {
			return err
		}
		cfg := `DEFAULT ipxe
PROMPT 0
TIMEOUT 0
LABEL ipxe
  KERNEL /ipxe.lkrn
  INITRD /embed.ipxe
`
		if err := os.WriteFile(filepath.Join(isoDir, "isolinux.cfg"), []byte(cfg), 0o644); err != nil {
			return err
		}
		bios = true
	}

	// README for operators
	readme := "rack-auto-ISO custom iPXE media\nUEFI: boots EFI/BOOT/BOOTX64.EFI (chains file:/embed.ipxe)\nBIOS: isolinux -> ipxe.lkrn + embed.ipxe initrd\n"
	_ = os.WriteFile(filepath.Join(tmp, "README.txt"), []byte(readme), 0o644)

	args := []string{
		"-as", "mkisofs",
		"-R", "-J", "-V", "IPXE",
		"-o", outPath,
	}
	if bios {
		args = append(args,
			"-b", "isolinux/isolinux.bin",
			"-c", "isolinux/boot.cat",
			"-no-emul-boot",
			"-boot-load-size", "4",
			"-boot-info-table",
		)
	}
	// UEFI El Torito
	args = append(args,
		"-eltorito-alt-boot",
		"-e", "EFI/BOOT/BOOTX64.EFI",
		"-no-emul-boot",
		tmp,
	)

	cmd := exec.Command(xorriso, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xorriso failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// BundleZIP packs embed script + boot assets for offline ISO crafting.
func BundleZIP(embedScript string, assetsDir string, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	ew, err := zw.Create("embed.ipxe")
	if err != nil {
		return err
	}
	if _, err := ew.Write([]byte(embedScript)); err != nil {
		return err
	}

	files := []string{AssetIPXELKRN, AssetIPXEEFI, AssetIsolinux, AssetLDLinux}
	for _, name := range files {
		src := filepath.Join(assetsDir, name)
		if !fileExists(src) {
			continue
		}
		if err := addFileToZip(zw, src, name); err != nil {
			return err
		}
	}

	readme := `Custom iPXE boot bundle from rack-auto-ISO

UEFI:
  Use ipxe.efi as EFI/BOOT/BOOTX64.EFI on a FAT/ISO volume together with embed.ipxe
  (ipxe.efi is built to chain file:/embed.ipxe).

BIOS (with syslinux):
  isolinux.cfg:
    DEFAULT ipxe
    LABEL ipxe
      KERNEL /ipxe.lkrn
      INITRD /embed.ipxe

Or install xorriso and use the management UI "导出 iPXE ISO".
`
	rw, err := zw.Create("README.txt")
	if err != nil {
		return err
	}
	_, err = rw.Write([]byte(readme))
	return err
}

func addFileToZip(zw *zip.Writer, src, name string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
