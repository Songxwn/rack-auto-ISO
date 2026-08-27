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

func TestRawOverride(t *testing.T) {
	menu := model.Menu{RawScript: "echo hi\n"}
	out := ipxescript.MenuScript(menu, model.Settings{}, nil)
	if !strings.HasPrefix(out, "#!ipxe\n") {
		t.Fatal("should prepend shebang")
	}
}
