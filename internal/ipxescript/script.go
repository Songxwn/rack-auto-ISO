package ipxescript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Songxwn/rack-auto-ISO/internal/ascii"
	"github.com/Songxwn/rack-auto-ISO/internal/isoprep"
	"github.com/Songxwn/rack-auto-ISO/internal/model"
)

// EmbedScript builds the script baked into exported iPXE ISO.
func EmbedScript(settings model.Settings) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n")
	b.WriteString(fmt.Sprintf("echo %s - network bootstrap\n", safe(settings.ServerName)))
	b.WriteString(networkBlock(settings.ISONet))
	chain := settings.ChainURL
	if chain == "" {
		base := strings.TrimRight(settings.PublicURL, "/")
		if base == "" {
			base = "http://${next-server}:8081"
		}
		chain = base + "/boot.ipxe"
	}
	b.WriteString(":chain\n")
	b.WriteString(fmt.Sprintf("echo Chaining to %s\n", chain))
	b.WriteString(fmt.Sprintf("chain --autofree %s || goto failed\n", chain))
	b.WriteString("goto shell\n")
	b.WriteString(":failed\n")
	b.WriteString("echo Chain failed - dropping to shell\n")
	b.WriteString(":shell\n")
	b.WriteString("shell\n")
	return b.String()
}

// BootScript is the entry script served at /boot.ipxe.
func BootScript(settings model.Settings, menuID string) string {
	base := strings.TrimRight(settings.PublicURL, "/")
	menuURL := "/menu.ipxe"
	if menuID != "" && menuID != "default" {
		menuURL = "/menu.ipxe?id=" + menuID
	}
	if base != "" {
		menuURL = base + menuURL
	}

	var b strings.Builder
	b.WriteString("#!ipxe\n")
	b.WriteString(fmt.Sprintf("echo %s\n", safe(settings.ServerName)))
	b.WriteString(networkBlock(settings.DefaultNet))
	b.WriteString(fmt.Sprintf("chain --autofree %s || shell\n", menuURL))
	return b.String()
}

// AssetsURL is where wimboot and similar helpers are served (e.g. /files/assets).
type BootPaths struct {
	PublicBase string // http://host:8081
	AssetsBase string // http://host:8081/files/assets
	BootBase   string // http://host:8081/files/boot
	ISOBase    string // http://host:8081/files/isos
	Wimboot    string // full URL to wimboot, optional
}

// MenuScript renders a menu (or returns raw script override).
func MenuScript(menu model.Menu, settings model.Settings, isos []model.ISOFile, paths BootPaths) string {
	if strings.TrimSpace(menu.RawScript) != "" {
		raw := menu.RawScript
		if !strings.HasPrefix(strings.TrimSpace(raw), "#!ipxe") {
			raw = "#!ipxe\n" + raw
		}
		return raw
	}

	base := strings.TrimRight(settings.PublicURL, "/")
	if paths.PublicBase == "" {
		paths.PublicBase = base
	}
	if paths.ISOBase == "" && base != "" {
		paths.ISOBase = base + "/files/isos"
	}
	if paths.BootBase == "" && base != "" {
		paths.BootBase = base + "/files/boot"
	}
	if paths.AssetsBase == "" && base != "" {
		paths.AssetsBase = base + "/files/assets"
	}
	if paths.Wimboot == "" && paths.AssetsBase != "" {
		paths.Wimboot = paths.AssetsBase + "/wimboot"
	}

	isoByID := map[string]model.ISOFile{}
	for _, f := range isos {
		isoByID[f.ID] = f
	}

	var b strings.Builder
	b.WriteString("#!ipxe\n")
	timeout := menu.TimeoutSec
	if timeout <= 0 {
		timeout = 30
	}
	b.WriteString(fmt.Sprintf("set menu-timeout %d000\n", timeout))
	b.WriteString("set submenu-timeout ${menu-timeout}\n")
	def := menu.DefaultItem
	if def == "" {
		for _, it := range menu.Items {
			if it.Enabled {
				def = it.ID
				break
			}
		}
	}
	if def != "" {
		b.WriteString(fmt.Sprintf("isset ${menu-default} || set menu-default %s\n", def))
	} else {
		b.WriteString("isset ${menu-default} || set menu-default shell\n")
	}

	b.WriteString(":start\n")
	b.WriteString("menu ")
	b.WriteString(escapeMenu(menu.Name))
	b.WriteString("\n")
	if menu.Description != "" {
		b.WriteString("item --gap -- ")
		b.WriteString(escapeMenu(menu.Description))
		b.WriteString("\n")
		b.WriteString("item --gap -- --------------------------------\n")
	}
	for _, it := range menu.Items {
		if !it.Enabled {
			continue
		}
		b.WriteString(fmt.Sprintf("item %s %s\n", it.ID, escapeMenu(it.Label)))
	}
	b.WriteString("choose --timeout ${menu-timeout} --default ${menu-default} selected || goto cancel\n")
	b.WriteString("set menu-timeout 0\n")
	b.WriteString("goto ${selected}\n\n")

	for _, it := range menu.Items {
		if !it.Enabled {
			continue
		}
		b.WriteString(fmt.Sprintf(":%s\n", it.ID))
		switch it.Type {
		case model.ItemShell:
			b.WriteString("shell\n")
		case model.ItemExit:
			b.WriteString("exit\n")
		case model.ItemChain:
			url := it.URL
			if url == "" {
				b.WriteString("echo missing chain url\n")
				b.WriteString("sleep 3\n")
				b.WriteString("goto start\n")
			} else {
				b.WriteString(fmt.Sprintf("chain --autofree %s || goto start\n", url))
			}
		case model.ItemSanboot:
			url := it.URL
			if url == "" {
				b.WriteString("echo missing sanboot url\n")
				b.WriteString("sleep 3\n")
				b.WriteString("goto start\n")
			} else {
				b.WriteString(resetVideoBeforeOS())
				b.WriteString(fmt.Sprintf("sanboot %s || goto start\n", url))
			}
		case model.ItemISO:
			f, ok := isoByID[it.ISOID]
			if !ok {
				b.WriteString("echo ISO not found\n")
				b.WriteString("sleep 3\n")
				b.WriteString("goto start\n")
				break
			}
			b.WriteString(bootISOItem(f, paths, effectiveVLAN(settings)))
		case model.ItemKernel:
			if it.Kernel == "" {
				b.WriteString("echo missing kernel\n")
				b.WriteString("sleep 3\n")
				b.WriteString("goto start\n")
				break
			}
			b.WriteString(resetVideoBeforeOS())
			args := strings.TrimSpace(it.Args)
			if args == "" {
				args = "nomodeset vga=normal"
			}
			b.WriteString(fmt.Sprintf("kernel %s %s\n", it.Kernel, args))
			if it.Initrd != "" {
				b.WriteString(fmt.Sprintf("initrd %s\n", it.Initrd))
			}
			b.WriteString("boot || goto start\n")
		case model.ItemCustom:
			custom := strings.TrimSpace(it.Custom)
			if custom == "" {
				b.WriteString("echo empty custom item\n")
				b.WriteString("sleep 2\n")
				b.WriteString("goto start\n")
			} else {
				b.WriteString(custom)
				if !strings.HasSuffix(custom, "\n") {
					b.WriteString("\n")
				}
			}
		default:
			b.WriteString("echo unknown item type\n")
			b.WriteString("sleep 2\n")
			b.WriteString("goto start\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(":cancel\n")
	b.WriteString("echo Cancelled\n")
	b.WriteString("sleep 1\n")
	b.WriteString("goto start\n")
	return b.String()
}

func effectiveVLAN(settings model.Settings) int {
	if settings.DefaultNet.VLAN >= 1 && settings.DefaultNet.VLAN <= 4094 {
		return settings.DefaultNet.VLAN
	}
	if settings.ISONet.VLAN >= 1 && settings.ISONet.VLAN <= 4094 {
		return settings.ISONet.VLAN
	}
	return 0
}

func bootISOItem(f model.ISOFile, paths BootPaths, vlan int) string {
	var b strings.Builder
	isoURL := fmt.Sprintf("%s/%s", strings.TrimRight(paths.ISOBase, "/"), f.Filename)
	bootURL := fmt.Sprintf("%s/%s", strings.TrimRight(paths.BootBase, "/"), f.ID)
	b.WriteString(fmt.Sprintf("echo Booting %s (%s)\n", escapeMenu(f.Name), escapeMenu(string(f.Distro))))
	b.WriteString(resetVideoBeforeOS())

	method := f.BootMethod
	if !f.PrepOK {
		method = "sanboot"
	}
	// Debian d-i needs the ISO as a CD/DVD device; kernel-only boot has no media.
	if f.Distro == model.DistroDebian {
		method = "sanboot"
	}
	vlanArgs := vlanKernelArgs(vlan)

	switch method {
	case "kernel-repo":
		b.WriteString(fmt.Sprintf("kernel %s/images/pxeboot/vmlinuz inst.repo=%s ip=dhcp nomodeset inst.graphical=0%s\n", bootURL, isoURL, vlanArgs))
		b.WriteString(fmt.Sprintf("initrd %s/images/pxeboot/initrd.img\n", bootURL))
		b.WriteString("boot || goto start\n")
	case "esxi-mboot":
		if f.PrepDir != "" {
			_ = isoprep.ApplyESXiPrefix(f.PrepDir, bootURL)
		}
		b.WriteString("iseq ${platform} efi && goto esxi_efi || goto esxi_bios\n")
		b.WriteString(":esxi_efi\n")
		b.WriteString(fmt.Sprintf("kernel %s/efi/boot/bootx64.efi -c %s/boot.cfg || kernel %s/bootx64.efi -c %s/boot.cfg\n", bootURL, bootURL, bootURL, bootURL))
		b.WriteString("boot || goto esxi_bios\n")
		b.WriteString(":esxi_bios\n")
		b.WriteString(fmt.Sprintf("sanboot --no-describe %s || goto start\n", isoURL))
	case "wimboot":
		wim := paths.Wimboot
		if wim == "" {
			b.WriteString("echo wimboot asset missing\n")
			b.WriteString("sleep 3\n")
			b.WriteString("goto start\n")
			break
		}
		bcd := "Boot/BCD"
		sdi := "Boot/boot.sdi"
		if prepHas(f.PrepDir, "bcd") || prepHas(f.PrepDir, "BCD") {
			if prepRel(f.PrepDir, "BCD") != "" {
				bcd = prepRel(f.PrepDir, "BCD")
			} else if prepRel(f.PrepDir, "bcd") != "" {
				bcd = prepRel(f.PrepDir, "bcd")
			}
			if prepRel(f.PrepDir, "boot.sdi") != "" {
				sdi = prepRel(f.PrepDir, "boot.sdi")
			}
		}
		b.WriteString("iseq ${platform} efi && goto win_efi || goto win_bios\n")
		b.WriteString(":win_efi\n")
		b.WriteString(fmt.Sprintf("kernel %s\n", wim))
		b.WriteString(fmt.Sprintf("initrd -n bootmgfw.efi %s/efi/boot/bootx64.efi ||\n", bootURL))
		b.WriteString(fmt.Sprintf("initrd -n bootmgr.efi %s/bootmgr.efi ||\n", bootURL))
		b.WriteString(fmt.Sprintf("initrd -n BCD %s/%s\n", bootURL, bcd))
		b.WriteString(fmt.Sprintf("initrd -n boot.sdi %s/%s\n", bootURL, sdi))
		b.WriteString(fmt.Sprintf("initrd -n boot.wim %s/sources/boot.wim\n", bootURL))
		b.WriteString("boot || goto start\n")
		b.WriteString(":win_bios\n")
		b.WriteString(fmt.Sprintf("kernel %s\n", wim))
		b.WriteString(fmt.Sprintf("initrd -n bootmgr.exe %s/bootmgr || initrd -n bootmgr %s/bootmgr\n", bootURL, bootURL))
		b.WriteString(fmt.Sprintf("initrd -n BCD %s/%s\n", bootURL, bcd))
		b.WriteString(fmt.Sprintf("initrd -n boot.sdi %s/%s\n", bootURL, sdi))
		b.WriteString(fmt.Sprintf("initrd -n boot.wim %s/sources/boot.wim\n", bootURL))
		b.WriteString("boot || goto start\n")
	case "debian-kernel":
		// Legacy method: kernel/initrd alone has no CD. Prefer sanboot of the ISO.
		b.WriteString("echo Debian needs ISO media - trying sanboot\n")
		b.WriteString(fmt.Sprintf("sanboot --no-describe %s || goto debian_net\n", isoURL))
		b.WriteString(":debian_net\n")
		vmlinuz := "install.amd/vmlinuz"
		initrd := "install.amd/initrd.gz"
		if prepRel(f.PrepDir, "vmlinuz") != "" {
			if r := findRel(f.PrepDir, "vmlinuz"); r != "" {
				vmlinuz = r
			}
			if r := findRel(f.PrepDir, "initrd.gz"); r != "" {
				initrd = r
			}
		}
		// Network mirror fallback (needs Internet or a local Debian mirror).
		host, dir := mirrorFromPublic(paths.PublicBase)
		b.WriteString(fmt.Sprintf("kernel %s/%s vga=normal nomodeset%s mirror/country=manual mirror/protocol=http mirror/http/hostname=%s mirror/http/directory=%s mirror/suite=stable cdrom-detect/failed=true --- quiet\n",
			bootURL, vmlinuz, vlanArgs, host, dir))
		b.WriteString(fmt.Sprintf("initrd %s/%s\n", bootURL, initrd))
		b.WriteString("boot || goto start\n")
	case "ubuntu-kernel":
		// Provide ISO URL for casper; also try sanboot if live rootfs fetch fails later.
		b.WriteString(fmt.Sprintf("kernel %s/casper/vmlinuz boot=casper url=%s ignore_uuid only-ubiquity nomodeset%s ---\n", bootURL, isoURL, vlanArgs))
		if prepHas(f.PrepDir, "initrd.lz") {
			b.WriteString(fmt.Sprintf("initrd %s/casper/initrd.lz\n", bootURL))
		} else {
			b.WriteString(fmt.Sprintf("initrd %s/casper/initrd\n", bootURL))
		}
		b.WriteString("boot || goto start\n")
	default:
		// Including DistroDebian with BootMethod sanboot
		b.WriteString(fmt.Sprintf("sanboot --no-describe %s || goto start\n", isoURL))
	}
	return b.String()
}

func mirrorFromPublic(publicBase string) (host, dir string) {
	host = "deb.debian.org"
	dir = "/debian"
	publicBase = strings.TrimSpace(publicBase)
	if publicBase == "" {
		return host, dir
	}
	// http://192.168.1.10:8081 → use that host with /debian if user runs a mirror;
	// otherwise keep deb.debian.org (sanboot path is preferred for local ISO).
	u := publicBase
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	if u != "" {
		// Still default directory to official mirror path; local ISO is via sanboot.
		_ = u
	}
	return host, dir
}

func prepHas(prepDir, name string) bool {
	if prepDir == "" {
		return false
	}
	return findRel(prepDir, name) != ""
}

func prepRel(prepDir, name string) string {
	return findRel(prepDir, name)
}

func findRel(root, name string) string {
	if root == "" {
		return ""
	}
	var rel string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.EqualFold(info.Name(), name) {
			r, err := filepath.Rel(root, path)
			if err == nil {
				rel = filepath.ToSlash(r)
			}
			return filepath.SkipAll
		}
		return nil
	})
	return rel
}

func resetVideoBeforeOS() string {
	return "console ||\nimgfree ||\n"
}

func networkBlock(net model.NetworkConfig) string {
	var b strings.Builder
	iface := "net0"
	vlan := net.VLAN
	if vlan < 0 || vlan > 4094 {
		vlan = 0
	}

	b.WriteString(":netconf\n")
	if vlan >= 1 {
		// Tagged VLAN: vcreate net0-<tag>, then DHCP/static on that interface.
		b.WriteString(fmt.Sprintf("echo VLAN %d on net0\n", vlan))
		b.WriteString(fmt.Sprintf("vcreate --tag %d net0 || goto netfail\n", vlan))
		iface = fmt.Sprintf("net0-%d", vlan)
	}

	switch net.Mode {
	case model.NetStatic:
		if net.IP != "" {
			b.WriteString(fmt.Sprintf("set %s/ip %s\n", iface, net.IP))
		}
		if net.Netmask != "" {
			b.WriteString(fmt.Sprintf("set %s/netmask %s\n", iface, net.Netmask))
		}
		if net.Gateway != "" {
			b.WriteString(fmt.Sprintf("set %s/gateway %s\n", iface, net.Gateway))
		}
		if net.DNS != "" {
			b.WriteString(fmt.Sprintf("set %s/dns %s\n", iface, net.DNS))
		}
		b.WriteString(fmt.Sprintf("ifopen %s || goto netfail\n", iface))
	default:
		b.WriteString(fmt.Sprintf("dhcp %s || goto netfail\n", iface))
	}
	b.WriteString("goto netok\n")
	b.WriteString(":netfail\n")
	b.WriteString("echo Network config failed\n")
	if vlan >= 1 {
		b.WriteString(fmt.Sprintf("echo Check trunk port allows VLAN %d\n", vlan))
	}
	b.WriteString("prompt --key 0x197e --timeout 5000 Press F12 to retry ||\n")
	if vlan >= 1 {
		b.WriteString(fmt.Sprintf("vcreate --tag %d net0 ||\n", vlan))
		b.WriteString(fmt.Sprintf("dhcp net0-%d || shell\n", vlan))
	} else {
		b.WriteString("dhcp || shell\n")
	}
	b.WriteString(":netok\n")
	return b.String()
}

// vlanKernelArgs helps Anaconda/dracut keep using the tagged VLAN after iPXE handoff.
func vlanKernelArgs(vlan int) string {
	if vlan < 1 || vlan > 4094 {
		return ""
	}
	// Prefer bootif-based naming where possible; also provide eth0.<vlan> style.
	return fmt.Sprintf(" vlan=eth0.%d:eth0 ip=eth0.%d:dhcp ", vlan, vlan)
}

func escapeMenu(s string) string {
	s = ascii.MenuText(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if s == "" {
		return "-"
	}
	return s
}

func safe(s string) string {
	if s == "" {
		return "ipxe-manager"
	}
	return escapeMenu(s)
}
