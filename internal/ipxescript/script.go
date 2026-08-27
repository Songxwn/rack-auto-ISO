package ipxescript

import (
	"fmt"
	"strings"

	"github.com/Songxwn/rack-auto-ISO/internal/ascii"
	"github.com/Songxwn/rack-auto-ISO/internal/model"
)

// EmbedScript builds the script baked into exported iPXE ISO.
// It configures network then chains to the management server menu.
func EmbedScript(settings model.Settings) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n")
	b.WriteString("set console console,vga\n")
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
	if base == "" {
		base = ""
	}
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

// MenuScript renders a menu (or returns raw script override).
func MenuScript(menu model.Menu, settings model.Settings, isos []model.ISOFile) string {
	if strings.TrimSpace(menu.RawScript) != "" {
		raw := menu.RawScript
		if !strings.HasPrefix(strings.TrimSpace(raw), "#!ipxe") {
			raw = "#!ipxe\n" + raw
		}
		return raw
	}

	base := strings.TrimRight(settings.PublicURL, "/")
	isoByID := map[string]model.ISOFile{}
	for _, f := range isos {
		isoByID[f.ID] = f
	}

	var b strings.Builder
	b.WriteString("#!ipxe\n")
	b.WriteString("console --x 1024 --y 768 ||\n")
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
		case model.ItemKernel:
			if it.Kernel == "" {
				b.WriteString("echo missing kernel\n")
				b.WriteString("sleep 3\n")
				b.WriteString("goto start\n")
				break
			}
			b.WriteString(fmt.Sprintf("kernel %s %s\n", it.Kernel, it.Args))
			if it.Initrd != "" {
				b.WriteString(fmt.Sprintf("initrd %s\n", it.Initrd))
			}
			b.WriteString("boot || goto start\n")
		case model.ItemSanboot:
			url := it.URL
			if url == "" {
				b.WriteString("echo missing sanboot url\n")
				b.WriteString("sleep 3\n")
				b.WriteString("goto start\n")
			} else {
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
			url := it.URL
			if url == "" {
				if base != "" {
					url = fmt.Sprintf("%s/files/isos/%s", base, f.Filename)
				} else {
					url = fmt.Sprintf("isos/%s", f.Filename)
				}
			}
			// http sanboot works for many Linux live ISOs when iPXE has the feature
			b.WriteString(fmt.Sprintf("echo Booting ISO %s\n", escapeMenu(f.Name)))
			b.WriteString(fmt.Sprintf("sanboot --no-describe %s || goto start\n", url))
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

func networkBlock(net model.NetworkConfig) string {
	var b strings.Builder
	switch net.Mode {
	case model.NetStatic:
		b.WriteString(":netconf\n")
		if net.IP != "" {
			b.WriteString(fmt.Sprintf("set net0/ip %s\n", net.IP))
		}
		if net.Netmask != "" {
			b.WriteString(fmt.Sprintf("set net0/netmask %s\n", net.Netmask))
		}
		if net.Gateway != "" {
			b.WriteString(fmt.Sprintf("set net0/gateway %s\n", net.Gateway))
		}
		if net.DNS != "" {
			b.WriteString(fmt.Sprintf("set net0/dns %s\n", net.DNS))
		}
		b.WriteString("ifopen net0 || goto netfail\n")
	default:
		b.WriteString(":netconf\n")
		b.WriteString("dhcp || goto netfail\n")
	}
	b.WriteString("goto netok\n")
	b.WriteString(":netfail\n")
	b.WriteString("echo Network config failed\n")
	b.WriteString("prompt --key 0x197e --timeout 5000 Press F12 to retry DHCP ||\n")
	b.WriteString("dhcp || shell\n")
	b.WriteString(":netok\n")
	return b.String()
}

func escapeMenu(s string) string {
	// iPXE menu/console is ASCII-oriented; strip CJK and control chars.
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
