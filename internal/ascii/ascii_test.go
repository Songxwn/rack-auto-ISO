package ascii_test

import (
	"testing"

	"github.com/Songxwn/rack-auto-ISO/internal/ascii"
)

func TestMenuTextStripsCJK(t *testing.T) {
	got := ascii.MenuText("启动菜单 Boot Menu 测试")
	if got != "Boot Menu" {
		t.Fatalf("got %q", got)
	}
}

func TestMenuLabelFromChineseName(t *testing.T) {
	got := ascii.MenuLabelFromName("系统镜像.iso", "abcd1234-xxxx")
	if got != "ISO-abcd1234" {
		t.Fatalf("got %q", got)
	}
}

func TestMenuLabelFromASCII(t *testing.T) {
	got := ascii.MenuLabelFromName("ubuntu-22.04-live-server-amd64.iso", "x")
	if got != "ubuntu-22.04-live-server-amd64" {
		t.Fatalf("got %q", got)
	}
}
