package model

import "time"

type NetMode string

const (
	NetDHCP   NetMode = "dhcp"
	NetStatic NetMode = "static"
)

type NetworkConfig struct {
	Mode    NetMode `json:"mode"`
	VLAN    int     `json:"vlan,omitempty"` // 0=off; 1-4094 creates net0-<vlan> then DHCP/static
	IP      string  `json:"ip,omitempty"`
	Netmask string  `json:"netmask,omitempty"`
	Gateway string  `json:"gateway,omitempty"`
	DNS     string  `json:"dns,omitempty"`
}

type Settings struct {
	PublicURL  string        `json:"publicUrl"`
	ServerName string        `json:"serverName"`
	DefaultNet NetworkConfig `json:"defaultNetwork"`
	ISONet     NetworkConfig `json:"isoNetwork"`
	ChainURL   string        `json:"chainUrl"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

type MenuItemType string

const (
	ItemChain   MenuItemType = "chain"
	ItemKernel  MenuItemType = "kernel"
	ItemSanboot MenuItemType = "sanboot"
	ItemISO     MenuItemType = "iso"
	ItemShell   MenuItemType = "shell"
	ItemExit    MenuItemType = "exit"
	ItemCustom  MenuItemType = "custom"
)

type MenuItem struct {
	ID      string       `json:"id"`
	Label   string       `json:"label"`
	Type    MenuItemType `json:"type"`
	URL     string       `json:"url,omitempty"`
	Kernel  string       `json:"kernel,omitempty"`
	Initrd  string       `json:"initrd,omitempty"`
	Args    string       `json:"args,omitempty"`
	ISOID   string       `json:"isoId,omitempty"`
	Custom  string       `json:"custom,omitempty"`
	Enabled bool         `json:"enabled"`
}

type Menu struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	TimeoutSec  int        `json:"timeoutSec"`
	DefaultItem string     `json:"defaultItem,omitempty"`
	Items       []MenuItem `json:"items"`
	RawScript   string     `json:"rawScript,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// DistroKind is detected from ISO contents for tailored iPXE boot.
type DistroKind string

const (
	DistroGeneric DistroKind = "generic"
	DistroRHEL    DistroKind = "rhel"    // RHEL 8-10 / Rocky / Alma / CentOS Stream / Oracle
	DistroESXi    DistroKind = "esxi"    // ESXi 6.7 - 9.x
	DistroWindows DistroKind = "windows" // Windows installer ISO
	DistroDebian  DistroKind = "debian"  // Debian installer
	DistroUbuntu  DistroKind = "ubuntu"  // Ubuntu live/installer
)

type ISOFile struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Filename    string     `json:"filename"`
	Size        int64      `json:"size"`
	ContentType string     `json:"contentType,omitempty"`
	UploadedAt  time.Time  `json:"uploadedAt"`
	Note        string     `json:"note,omitempty"`
	Distro      DistroKind `json:"distro,omitempty"`
	BootMethod  string     `json:"bootMethod,omitempty"` // kernel-repo | esxi-mboot | wimboot | sanboot | debian-kernel
	PrepDir     string     `json:"prepDir,omitempty"`    // relative under data/boot/<id>
	PrepOK      bool       `json:"prepOk"`
	PrepError   string     `json:"prepError,omitempty"`
}

type State struct {
	Settings Settings  `json:"settings"`
	Menus    []Menu    `json:"menus"`
	ISOs     []ISOFile `json:"isos"`
}
