package ipxescript_test

import (
	"strings"
	"testing"

	"github.com/Songxwn/rack-auto-ISO/internal/ipxescript"
	"github.com/Songxwn/rack-auto-ISO/internal/model"
)

func TestEmbedScriptDHCP(t *testing.T) {
	s := ipxescript.EmbedScript(model.Settings{
		ServerName: "test",
		PublicURL:  "http://10.0.0.1:8081",
		ISONet:     model.NetworkConfig{Mode: model.NetDHCP},
	})
	if !strings.HasPrefix(s, "#!ipxe") {
		t.Fatal("missing shebang")
	}
	if !strings.Contains(s, "dhcp") {
		t.Fatal("expected dhcp")
	}
	if !strings.Contains(s, "http://10.0.0.1:8081/boot.ipxe") {
		t.Fatalf("chain url missing: %s", s)
	}
}

func TestMenuScriptItems(t *testing.T) {
	menu := model.Menu{
		Name:       "Demo",
		TimeoutSec: 10,
		Items: []model.MenuItem{
			{ID: "a", Label: "Shell", Type: model.ItemShell, Enabled: true},
			{ID: "b", Label: "Off", Type: model.ItemExit, Enabled: false},
		},
	}
	out := ipxescript.MenuScript(menu, model.Settings{}, nil, ipxescript.BootPaths{})
	if !strings.Contains(out, "item a Shell") {
		t.Fatal("missing item")
	}
	if strings.Contains(out, "item b ") {
		t.Fatal("disabled item should be omitted")
	}
}

func TestMenuStripsChinese(t *testing.T) {
	menu := model.Menu{
		Name: "默认 Boot",
		Items: []model.MenuItem{
			{ID: "a", Label: "安装 Install", Type: model.ItemShell, Enabled: true},
		},
	}
	out := ipxescript.MenuScript(menu, model.Settings{}, nil, ipxescript.BootPaths{})
	if strings.Contains(out, "默认") || strings.Contains(out, "安装") {
		t.Fatalf("chinese leaked: %s", out)
	}
}

func TestRHELBootScript(t *testing.T) {
	menu := model.Menu{
		Name: "Boot Menu",
		Items: []model.MenuItem{
			{ID: "r", Label: "RHEL", Type: model.ItemISO, ISOID: "id1", Enabled: true},
		},
	}
	isos := []model.ISOFile{{
		ID: "id1", Name: "rhel.iso", Filename: "id1.iso",
		Distro: model.DistroRHEL, BootMethod: "kernel-repo", PrepOK: true,
	}}
	paths := ipxescript.BootPaths{
		ISOBase:  "http://10.0.0.1:8081/files/isos",
		BootBase: "http://10.0.0.1:8081/files/boot",
	}
	out := ipxescript.MenuScript(menu, model.Settings{PublicURL: "http://10.0.0.1:8081"}, isos, paths)
	if !strings.Contains(out, "inst.repo=http://10.0.0.1:8081/files/isos/id1.iso") {
		t.Fatalf("missing inst.repo: %s", out)
	}
	if !strings.Contains(out, "images/pxeboot/vmlinuz") {
		t.Fatal("missing vmlinuz")
	}
}

func TestWindowsWimboot(t *testing.T) {
	menu := model.Menu{
		Name: "Boot Menu",
		Items: []model.MenuItem{
			{ID: "w", Label: "Win", Type: model.ItemISO, ISOID: "idw", Enabled: true},
		},
	}
	isos := []model.ISOFile{{
		ID: "idw", Name: "win.iso", Filename: "idw.iso",
		Distro: model.DistroWindows, BootMethod: "wimboot", PrepOK: true,
	}}
	paths := ipxescript.BootPaths{
		ISOBase:  "http://10.0.0.1:8081/files/isos",
		BootBase: "http://10.0.0.1:8081/files/boot",
		Wimboot:  "http://10.0.0.1:8081/files/assets/wimboot",
	}
	out := ipxescript.MenuScript(menu, model.Settings{PublicURL: "http://10.0.0.1:8081"}, isos, paths)
	if !strings.Contains(out, "wimboot") || !strings.Contains(out, "boot.wim") {
		t.Fatalf("wimboot script incomplete: %s", out)
	}
	if !strings.Contains(out, "win_efi") {
		t.Fatal("expected UEFI/BIOS branches")
	}
}

func TestVLANEmbed(t *testing.T) {
	s := ipxescript.EmbedScript(model.Settings{
		ServerName: "test",
		PublicURL:  "http://10.0.0.1:8081",
		ISONet:     model.NetworkConfig{Mode: model.NetDHCP, VLAN: 100},
	})
	if !strings.Contains(s, "vcreate --tag 100 net0") {
		t.Fatalf("missing vcreate: %s", s)
	}
	if !strings.Contains(s, "dhcp net0-100") {
		t.Fatalf("missing vlan dhcp: %s", s)
	}
}

func TestRawOverride(t *testing.T) {
	menu := model.Menu{RawScript: "echo hi\n"}
	out := ipxescript.MenuScript(menu, model.Settings{}, nil, ipxescript.BootPaths{})
	if !strings.HasPrefix(out, "#!ipxe\n") {
		t.Fatal("should prepend shebang")
	}
}
