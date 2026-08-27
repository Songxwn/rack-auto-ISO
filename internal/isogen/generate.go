package isogen

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Asset names expected under AssetsDir (populated by CI).
const (
	AssetIPXELKRN   = "ipxe.lkrn"
	AssetIPXEEFI    = "ipxe.efi"
	AssetIsolinux   = "isolinux.bin"
	AssetLDLinux    = "ldlinux.c32"
	AssetIsohdpfx   = "isohdpfx.bin"
)

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
	need := []string{AssetIPXELKRN, AssetIPXEEFI, AssetIsolinux, AssetLDLinux}
	for _, n := range need {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			return false
		}
	}
	return true
}

// GenerateISO writes a BIOS+UEFI isohybrid ISO with custom embed.ipxe.
//
// Boot layout:
//   - BIOS: isolinux -> ipxe.lkrn + INITRD embed.ipxe
//   - UEFI: El Torito FAT efi.img (ESP) with BOOTX64.EFI (+ embed.ipxe)
//   - USB: isohybrid MBR + GPT basdat
//
// If IPXE_SRC is set (iPXE src dir) and make/gcc are available, rebuilds
// ipxe.efi with EMBED=the user script so UEFI does not depend on file:.
func GenerateISO(embedScript string, assetsDir, outPath string) error {
	if assetsDir == "" || !hasAssets(assetsDir) {
		return fmt.Errorf("iPXE boot assets incomplete under %q (need lkrn/efi/isolinux/ldlinux); use a Release or GHCR image that includes assets/", assetsDir)
	}
	if _, err := exec.LookPath("xorriso"); err != nil {
		return fmt.Errorf("xorriso not found in PATH: %w", err)
	}
	for _, bin := range []string{"mkfs.vfat", "mcopy", "mmd"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found (install dosfstools/mtools): %w", bin, err)
		}
	}

	tmp, err := os.MkdirTemp("", "ipxe-iso-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	root := filepath.Join(tmp, "iso")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	embedPath := filepath.Join(root, "embed.ipxe")
	if err := os.WriteFile(embedPath, []byte(embedScript), 0o644); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(assetsDir, AssetIPXELKRN), filepath.Join(root, AssetIPXELKRN)); err != nil {
		return err
	}

	// BIOS isolinux
	isoDir := filepath.Join(root, "isolinux")
	if err := os.MkdirAll(isoDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(assetsDir, AssetIsolinux), filepath.Join(isoDir, AssetIsolinux)); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(assetsDir, AssetLDLinux), filepath.Join(isoDir, AssetLDLinux)); err != nil {
		return err
	}
	cfg := `DEFAULT ipxe
PROMPT 0
TIMEOUT 0
SAY rack-auto-ISO BIOS boot
LABEL ipxe
  KERNEL /ipxe.lkrn
  INITRD /embed.ipxe
`
	if err := os.WriteFile(filepath.Join(isoDir, "isolinux.cfg"), []byte(cfg), 0o644); err != nil {
		return err
	}

	// UEFI: prefer freshly built ipxe.efi with user EMBED; else assets + file: chain
	efiBin := filepath.Join(tmp, "BOOTX64.EFI")
	rebuilt, err := rebuildEFI(embedPath, efiBin)
	if err != nil {
		// Non-fatal: fall back to prebuilt EFI that chains file:/embed.ipxe
		fmt.Fprintf(os.Stderr, "isogen: rebuild ipxe.efi failed, using assets fallback: %v\n", err)
		rebuilt = false
	}
	if !rebuilt {
		if err := copyFile(filepath.Join(assetsDir, AssetIPXEEFI), efiBin); err != nil {
			return err
		}
	}

	// Keep a copy on ISO9660 for clarity / some firmwares
	efiTree := filepath.Join(root, "EFI", "BOOT")
	if err := os.MkdirAll(efiTree, 0o755); err != nil {
		return err
	}
	if err := copyFile(efiBin, filepath.Join(efiTree, "BOOTX64.EFI")); err != nil {
		return err
	}

	efiImg := filepath.Join(root, "efi.img")
	if err := makeEFIImage(efiBin, embedPath, efiImg); err != nil {
		return fmt.Errorf("create efi.img: %w", err)
	}

	_ = os.WriteFile(filepath.Join(root, "README.txt"), []byte(
		"rack-auto-ISO custom iPXE media\n"+
			"BIOS: isolinux -> ipxe.lkrn + embed.ipxe (INITRD)\n"+
			"UEFI: efi.img ESP -> EFI/BOOT/BOOTX64.EFI (+ embed.ipxe on FAT)\n"+
			"USB: isohybrid MBR/GPT\n",
	), 0o644)

	args := []string{
		"-as", "mkisofs",
		"-R", "-J", "-joliet-long",
		"-V", "IPXE",
		"-o", outPath,
		"-b", "isolinux/isolinux.bin",
		"-c", "isolinux/boot.cat",
		"-no-emul-boot",
		"-boot-load-size", "4",
		"-boot-info-table",
	}
	if mbr := resolveIsohdpfx(assetsDir); mbr != "" {
		args = append(args, "-isohybrid-mbr", mbr)
	}
	args = append(args,
		"-eltorito-alt-boot",
		"-e", "efi.img",
		"-no-emul-boot",
		"-isohybrid-gpt-basdat",
		root,
	)

	cmd := exec.Command("xorriso", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xorriso failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func resolveIsohdpfx(assetsDir string) string {
	candidates := []string{
		filepath.Join(assetsDir, AssetIsohdpfx),
		"/usr/lib/ISOLINUX/isohdpfx.bin",
		"/usr/lib/syslinux/isohdpfx.bin",
		"/usr/share/syslinux/isohdpfx.bin",
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

// rebuildEFI builds ipxe.efi with EMBED=embedScript when IPXE_SRC is usable.
func rebuildEFI(embedPath, outEFI string) (bool, error) {
	src := strings.TrimSpace(os.Getenv("IPXE_SRC"))
	if src == "" {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(src, "Makefile")); err != nil {
		return false, nil
	}
	if _, err := exec.LookPath("make"); err != nil {
		return false, nil
	}

	jobs := "2"
	if n := os.Getenv("IPXE_MAKE_JOBS"); n != "" {
		jobs = n
	} else if out, err := exec.Command("nproc").Output(); err == nil {
		if j := strings.TrimSpace(string(out)); j != "" {
			jobs = j
		}
	}

	cmd := exec.Command("make", "-j"+jobs, "bin-x86_64-efi/ipxe.efi", "EMBED="+embedPath)
	cmd.Dir = src
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	built := filepath.Join(src, "bin-x86_64-efi", "ipxe.efi")
	if err := copyFile(built, outEFI); err != nil {
		return false, err
	}
	return true, nil
}

func makeEFIImage(efiBinary, embedPath, outImg string) error {
	// Size: ipxe.efi ~1–2MiB + embed; 8MiB FAT is safe for El Torito.
	const sizeMB = 8
	if err := exec.Command("dd", "if=/dev/zero", "of="+outImg, "bs=1M", "count="+strconv.Itoa(sizeMB), "status=none").Run(); err != nil {
		// busybox/dd without status=
		if err2 := exec.Command("dd", "if=/dev/zero", "of="+outImg, "bs=1M", "count="+strconv.Itoa(sizeMB)).Run(); err2 != nil {
			return fmt.Errorf("dd efi.img: %v", err2)
		}
	}
	if out, err := exec.Command("mkfs.vfat", "-n", "IPXE_ESP", outImg).CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.vfat: %v: %s", err, strings.TrimSpace(string(out)))
	}
	// Create EFI/BOOT directories inside the FAT image
	for _, args := range [][]string{
		{"mmd", "-i", outImg, "::/EFI"},
		{"mmd", "-i", outImg, "::/EFI/BOOT"},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.Command("mcopy", "-i", outImg, efiBinary, "::/EFI/BOOT/BOOTX64.EFI").CombinedOutput(); err != nil {
		return fmt.Errorf("mcopy BOOTX64.EFI: %v: %s", err, strings.TrimSpace(string(out)))
	}
	// Also place embed.ipxe on ESP root for file:/embed.ipxe fallback builds
	if out, err := exec.Command("mcopy", "-i", outImg, embedPath, "::/embed.ipxe").CombinedOutput(); err != nil {
		return fmt.Errorf("mcopy embed.ipxe: %v: %s", err, strings.TrimSpace(string(out)))
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

	files := []string{AssetIPXELKRN, AssetIPXEEFI, AssetIsolinux, AssetLDLinux, AssetIsohdpfx}
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

Prefer exporting ISO from the management UI / GHCR image (proper isohybrid).

Manual UEFI: put ipxe.efi as EFI/BOOT/BOOTX64.EFI on a FAT ESP together with embed.ipxe.
Manual BIOS: isolinux KERNEL /ipxe.lkrn INITRD /embed.ipxe
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
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
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
