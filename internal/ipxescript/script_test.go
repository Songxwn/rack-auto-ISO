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
		PublicURL:  "http://10.0.0.1:8080",
		ISONet:     model.NetworkConfig{Mode: model.NetDHCP},
	})
	if !strings.HasPrefix(s, "#!ipxe") {
		t.Fatal("missing shebang")
	}
	if !strings.Contains(s, "dhcp") {
		t.Fatal("expected dhcp")
	}
	if !strings.Contains(s, "http://10.0.0.1:8080/boot.ipxe") {
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
	out := ipxescript.MenuScript(menu, model.Settings{}, nil)
	if !strings.Contains(out, "item a Shell") {
		t.Fatal("missing item")
	}
	if strings.Contains(out, "item b ") {
		t.Fatal("disabled item should be omitted")
	}
	if !strings.Contains(out, ":a\nshell") {
		t.Fatal("missing shell target")
	}
}

func TestMenuStripsChinese(t *testing.T) {
	menu := model.Menu{
		Name:        "默认启动菜单 Boot",
		Description: "机架装机",
		Items: []model.MenuItem{
			{ID: "a", Label: "安装系统 Install", Type: model.ItemShell, Enabled: true},
		},
	}
	out := ipxescript.MenuScript(menu, model.Settings{}, nil)
	if strings.Contains(out, "默认") || strings.Contains(out, "机架") || strings.Contains(out, "安装") {
		t.Fatalf("chinese leaked into script: %s", out)
	}
	if !strings.Contains(out, "menu Boot") {
		t.Fatalf("expected ASCII menu title: %s", out)
	}
	if !strings.Contains(out, "item a Install") {
		t.Fatalf("expected ASCII item label: %s", out)
	}
}

func TestISOResetsConsole(t *testing.T) {
	menu := model.Menu{
		Name: "Boot Menu",
		Items: []model.MenuItem{
			{ID: "d", Label: "Debian", Type: model.ItemISO, ISOID: "abc", Enabled: true},
		},
	}
	isos := []model.ISOFile{{ID: "abc", Name: "debian.iso", Filename: "abc.iso"}}
	out := ipxescript.MenuScript(menu, model.Settings{PublicURL: "http://10.0.0.1:8081"}, isos)
	if strings.Contains(out, "console --x") {
		t.Fatal("must not set graphical console resolution")
	}
	if !strings.Contains(out, "console ||") {
		t.Fatal("expected console reset before sanboot")
	}
	if !strings.Contains(out, "sanboot --no-describe http://10.0.0.1:8081/files/isos/abc.iso") {
		t.Fatalf("sanboot missing: %s", out)
	}
}

func TestRawOverride(t *testing.T) {
	menu := model.Menu{RawScript: "echo hi\n"}
	out := ipxescript.MenuScript(menu, model.Settings{}, nil)
	if !strings.HasPrefix(out, "#!ipxe\n") {
		t.Fatal("should prepend shebang")
	}
}
