package isoprep

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Songxwn/rack-auto-ISO/internal/model"
)

// Result of preparing an uploaded ISO for network boot.
type Result struct {
	Distro     model.DistroKind
	BootMethod string
	PrepDir    string // absolute path
	Error      string
}

// Prepare detects distro, extracts boot files into bootRoot/<isoID>/, returns metadata.
func Prepare(isoPath, bootRoot, isoID string) Result {
	res := Result{Distro: model.DistroGeneric, BootMethod: "sanboot"}
	outDir := filepath.Join(bootRoot, isoID)
	_ = os.RemoveAll(outDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		res.Error = err.Error()
		return res
	}
	res.PrepDir = outDir

	kind := Detect(isoPath)
	res.Distro = kind

	var err error
	switch kind {
	case model.DistroRHEL:
		err = prepRHEL(isoPath, outDir)
		res.BootMethod = "kernel-repo"
	case model.DistroESXi:
		err = prepESXi(isoPath, outDir)
		res.BootMethod = "esxi-mboot"
	case model.DistroWindows:
		err = prepWindows(isoPath, outDir)
		res.BootMethod = "wimboot"
	case model.DistroDebian:
		err = prepDebian(isoPath, outDir)
		res.BootMethod = "debian-mirror"
	case model.DistroUbuntu:
		err = prepUbuntu(isoPath, outDir)
		res.BootMethod = "ubuntu-kernel"
	default:
		res.BootMethod = "sanboot"
		err = nil
	}
	if err != nil {
		res.Error = err.Error()
		res.BootMethod = "sanboot"
		// keep distro label for UI even if prep failed
	}
	return res
}

// Detect probes ISO for known layout markers (via archive listing tools).
func Detect(isoPath string) model.DistroKind {
	listing := listISO(isoPath)
	lower := strings.ToLower(listing)

	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(lower, strings.ToLower(p)) {
				return true
			}
		}
		return false
	}

	// Order matters: more specific first
	if has("sources/boot.wim", "sources\\boot.wim", "sources/install.wim") {
		return model.DistroWindows
	}
	if has("boot.cfg") && (has("efi/boot/bootx64.efi") || has("mboot.c32") || has("b.b00") || has("s.v00")) {
		return model.DistroESXi
	}
	if has("images/pxeboot/vmlinuz") && (has(".treeinfo") || has("images/pxeboot/initrd.img") || has("baseos") || has("appstream")) {
		return model.DistroRHEL
	}
	if has("images/pxeboot/vmlinuz") && has("images/pxeboot/initrd.img") {
		return model.DistroRHEL
	}
	if has("casper/vmlinuz", "casper/initrd") {
		return model.DistroUbuntu
	}
	if has("install.amd/vmlinuz", "install.amd/initrd.gz", "install.a64/vmlinuz") {
		return model.DistroDebian
	}
	return model.DistroGeneric
}

func listISO(isoPath string) string {
	// Prefer bsdtar / 7z for Joliet+RockRidge listings
	if path, err := exec.LookPath("bsdtar"); err == nil {
		out, err := exec.Command(path, "-tf", isoPath).CombinedOutput()
		if err == nil {
			return string(out)
		}
	}
	if path, err := exec.LookPath("7z"); err == nil {
		out, err := exec.Command(path, "l", "-ba", isoPath).CombinedOutput()
		if err == nil {
			return string(out)
		}
	}
	if path, err := exec.LookPath("isoinfo"); err == nil {
		out, err := exec.Command(path, "-J", "-f", "-i", isoPath).CombinedOutput()
		if err == nil {
			return string(out)
		}
		out, _ = exec.Command(path, "-f", "-i", isoPath).CombinedOutput()
		return string(out)
	}
	return ""
}

func extract(isoPath, dest string, members ...string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if path, err := exec.LookPath("bsdtar"); err == nil {
		args := []string{"-xf", isoPath, "-C", dest}
		args = append(args, members...)
		if out, err := exec.Command(path, args...).CombinedOutput(); err != nil {
			return fmt.Errorf("bsdtar: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if path, err := exec.LookPath("7z"); err == nil {
		args := []string{"x", "-y", "-o" + dest, isoPath}
		args = append(args, members...)
		if out, err := exec.Command(path, args...).CombinedOutput(); err != nil {
			return fmt.Errorf("7z: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("need bsdtar (libarchive-tools) or 7z (p7zip-full) on the server to prepare this ISO")
}

func extractAll(isoPath, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if path, err := exec.LookPath("bsdtar"); err == nil {
		if out, err := exec.Command(path, "-xf", isoPath, "-C", dest).CombinedOutput(); err != nil {
			return fmt.Errorf("bsdtar: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if path, err := exec.LookPath("7z"); err == nil {
		if out, err := exec.Command(path, "x", "-y", "-o"+dest, isoPath).CombinedOutput(); err != nil {
			return fmt.Errorf("7z: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("need bsdtar or 7z to extract ISO")
}

func prepRHEL(isoPath, dest string) error {
	// Minimal pxeboot payload; installation source remains the ISO URL (inst.repo=).
	err := extract(isoPath, dest,
		"images/pxeboot/vmlinuz",
		"images/pxeboot/initrd.img",
	)
	if err != nil {
		// some ISOs use different casing / paths
		err2 := extract(isoPath, dest, "IMAGES/PXEBOOT/VMLINUZ", "IMAGES/PXEBOOT/INITRD.IMG")
		if err2 != nil {
			return err
		}
	}
	if !fileExists(filepath.Join(dest, "images", "pxeboot", "vmlinuz")) {
		// 7z may flatten or use different structure — search
		if p := findFile(dest, "vmlinuz"); p != "" {
			_ = os.MkdirAll(filepath.Join(dest, "images", "pxeboot"), 0o755)
			_ = copyFile(p, filepath.Join(dest, "images", "pxeboot", "vmlinuz"))
		}
		if p := findFile(dest, "initrd.img"); p != "" {
			_ = os.MkdirAll(filepath.Join(dest, "images", "pxeboot"), 0o755)
			_ = copyFile(p, filepath.Join(dest, "images", "pxeboot", "initrd.img"))
		}
	}
	if !fileExists(filepath.Join(dest, "images", "pxeboot", "vmlinuz")) ||
		!fileExists(filepath.Join(dest, "images", "pxeboot", "initrd.img")) {
		return fmt.Errorf("RHEL pxeboot files missing after extract")
	}
	return nil
}

func prepESXi(isoPath, dest string) error {
	if err := extractAll(isoPath, dest); err != nil {
		return err
	}
	bootCfg := findFile(dest, "boot.cfg")
	if bootCfg == "" {
		return fmt.Errorf("ESXi boot.cfg not found")
	}
	// Placeholder PREFIX_URL replaced at menu render time via sidecar file
	if err := rewriteESXiBootCfg(bootCfg, "PREFIX_URL"); err != nil {
		return err
	}
	// Ensure we have UEFI loader
	efi := findFile(dest, "bootx64.efi")
	if efi == "" {
		return fmt.Errorf("ESXi bootx64.efi not found")
	}
	return nil
}

func rewriteESXiBootCfg(path, prefixURL string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var lines []string
	hasPrefix := false
	sc := bufio.NewScanner(f)
	// allow long modules= lines
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "prefix=") {
			lines = append(lines, "prefix="+prefixURL)
			hasPrefix = true
			continue
		}
		if strings.HasPrefix(lower, "kernel=") {
			v := strings.TrimPrefix(trim, trim[:strings.Index(trim, "=")+1])
			v = strings.TrimLeft(v, "/")
			lines = append(lines, "kernel="+v)
			continue
		}
		if strings.HasPrefix(lower, "modules=") {
			v := strings.TrimPrefix(trim, trim[:strings.Index(trim, "=")+1])
			parts := strings.Split(v, "---")
			for i := range parts {
				parts[i] = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(parts[i]), "/"))
			}
			lines = append(lines, "modules="+strings.Join(parts, " --- "))
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if !hasPrefix {
		lines = append([]string{"prefix=" + prefixURL}, lines...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// ApplyESXiPrefix sets prefix= in boot.cfg under prep dir to the HTTP base URL.
func ApplyESXiPrefix(prepDir, prefixURL string) error {
	bootCfg := findFile(prepDir, "boot.cfg")
	if bootCfg == "" {
		return fmt.Errorf("boot.cfg missing")
	}
	return rewriteESXiBootCfg(bootCfg, strings.TrimRight(prefixURL, "/"))
}

func prepWindows(isoPath, dest string) error {
	members := []string{
		"bootmgr",
		"bootmgr.efi",
		"Boot/BCD",
		"boot/bcd",
		"Boot/boot.sdi",
		"boot/boot.sdi",
		"sources/boot.wim",
		"efi/boot/bootx64.efi",
	}
	// Extract individually; ignore missing optional ones
	var errs []string
	for _, m := range members {
		if err := extract(isoPath, dest, m); err != nil {
			errs = append(errs, m+": "+err.Error())
		}
	}
	if findFile(dest, "boot.wim") == "" {
		return fmt.Errorf("windows boot.wim missing (%s)", strings.Join(errs, "; "))
	}
	if findFile(dest, "bcd") == "" && findFile(dest, "BCD") == "" {
		return fmt.Errorf("windows BCD missing")
	}
	return nil
}

func prepDebian(isoPath, dest string) error {
	// Full tree becomes a local HTTP mirror (dists/ + pool/).
	if err := extractAll(isoPath, dest); err != nil {
		return fmt.Errorf("extract debian iso: %w", err)
	}
	if findFile(dest, "vmlinuz") == "" {
		return fmt.Errorf("debian vmlinuz missing after extract")
	}
	suite := detectDebianSuite(dest)
	_ = os.WriteFile(filepath.Join(dest, ".suite"), []byte(suite+"\n"), 0o644)

	// Prefer official netboot initrd (skips cdrom-detect). Falls back to ISO's install.amd.
	if err := fetchDebianNetboot(dest, suite); err != nil {
		_ = os.WriteFile(filepath.Join(dest, ".netboot-error"), []byte(err.Error()), 0o644)
		// still OK — boot path can use install.amd with mirror= params
	}
	return nil
}

func detectDebianSuite(dest string) string {
	// dists/<codename>/Release
	dists := filepath.Join(dest, "dists")
	entries, err := os.ReadDir(dists)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			switch name {
			case "stable", "testing", "unstable", "oldstable":
				continue
			default:
				if fileExists(filepath.Join(dists, name, "Release")) {
					return name
				}
			}
		}
		for _, e := range entries {
			if e.IsDir() && fileExists(filepath.Join(dists, e.Name(), "Release")) {
				return e.Name()
			}
		}
	}
	// .disk/info — "Debian GNU/Linux 12.5.0 ..."
	if data, err := os.ReadFile(filepath.Join(dest, ".disk", "info")); err == nil {
		s := string(data)
		for _, code := range []string{"trixie", "bookworm", "bullseye", "buster", "sid"} {
			if strings.Contains(strings.ToLower(s), code) {
				return code
			}
		}
	}
	return "bookworm"
}

func fetchDebianNetboot(dest, suite string) error {
	if suite == "" {
		suite = "bookworm"
	}
	netDir := filepath.Join(dest, "netboot")
	if err := os.MkdirAll(netDir, 0o755); err != nil {
		return err
	}
	base := fmt.Sprintf(
		"https://deb.debian.org/debian/dists/%s/main/installer-amd64/current/images/netboot/debian-installer/amd64",
		suite,
	)
	if err := httpDownload(base+"/linux", filepath.Join(netDir, "linux")); err != nil {
		// try stable alias
		base2 := "https://deb.debian.org/debian/dists/stable/main/installer-amd64/current/images/netboot/debian-installer/amd64"
		if err2 := httpDownload(base2+"/linux", filepath.Join(netDir, "linux")); err2 != nil {
			return fmt.Errorf("netboot linux: %v / %v", err, err2)
		}
		base = base2
	}
	if err := httpDownload(base+"/initrd.gz", filepath.Join(netDir, "initrd.gz")); err != nil {
		return fmt.Errorf("netboot initrd: %w", err)
	}
	return nil
}

func httpDownload(url, dest string) error {
	cmd := exec.Command("curl", "-fsSL", "--connect-timeout", "20", "--max-time", "300", "-o", dest, url)
	if _, err := exec.LookPath("curl"); err != nil {
		cmd = exec.Command("wget", "-q", "-O", dest, url)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	fi, err := os.Stat(dest)
	if err != nil || fi.Size() < 1024 {
		return fmt.Errorf("download too small or missing: %s", dest)
	}
	return nil
}

func prepUbuntu(isoPath, dest string) error {
	err := extract(isoPath, dest, "casper/vmlinuz", "casper/initrd")
	if err != nil {
		err = extract(isoPath, dest, "casper/vmlinuz", "casper/initrd.lz")
	}
	if findFile(dest, "vmlinuz") == "" {
		return fmt.Errorf("ubuntu casper/vmlinuz missing: %v", err)
	}
	return nil
}

func findFile(root, name string) string {
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.EqualFold(info.Name(), name) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}
