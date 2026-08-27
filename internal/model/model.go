package model

import "time"

type NetMode string

const (
	NetDHCP   NetMode = "dhcp"
	NetStatic NetMode = "static"
)

type NetworkConfig struct {
	Mode    NetMode `json:"mode"`
	IP      string  `json:"ip,omitempty"`
	Netmask string  `json:"netmask,omitempty"`
	Gateway string  `json:"gateway,omitempty"`
	DNS     string  `json:"dns,omitempty"`
}

type Settings struct {
	PublicURL   string         `json:"publicUrl"`
	ServerName  string         `json:"serverName"`
	DefaultNet  NetworkConfig  `json:"defaultNetwork"`
	ISONet      NetworkConfig  `json:"isoNetwork"` // network config baked into exported iPXE ISO
	ChainURL    string         `json:"chainUrl"`   // override chain target; empty => {publicUrl}/boot.ipxe
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type MenuItemType string

const (
	ItemChain  MenuItemType = "chain"
	ItemKernel MenuItemType = "kernel"
	ItemSanboot MenuItemType = "sanboot"
	ItemISO    MenuItemType = "iso"
	ItemShell  MenuItemType = "shell"
	ItemExit   MenuItemType = "exit"
	ItemCustom MenuItemType = "custom"
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
	Custom  string       `json:"custom,omitempty"` // raw iPXE lines for custom type
	Enabled bool         `json:"enabled"`
}

type Menu struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	TimeoutSec  int        `json:"timeoutSec"`
	DefaultItem string     `json:"defaultItem,omitempty"`
	Items       []MenuItem `json:"items"`
	RawScript   string     `json:"rawScript,omitempty"` // if set, used as-is instead of generated menu
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type ISOFile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType,omitempty"`
	UploadedAt  time.Time `json:"uploadedAt"`
	Note        string    `json:"note,omitempty"`
}

type State struct {
	Settings Settings  `json:"settings"`
	Menus    []Menu    `json:"menus"`
	ISOs     []ISOFile `json:"isos"`
}
