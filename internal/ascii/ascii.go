package ascii

import (
	"path/filepath"
	"strings"
	"unicode"
)

// MenuText keeps printable ASCII only (iPXE menus do not render CJK reliably).
// Non-ASCII runes are dropped; whitespace is collapsed.
func MenuText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 0x20 && r < 0x7f:
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// MenuLabelFromName builds an iPXE-safe label from a display name / filename.
func MenuLabelFromName(name, fallbackID string) string {
	base := name
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".iso") {
		base = strings.TrimSuffix(base, ext)
	}
	label := MenuText(base)
	label = strings.Trim(label, ".-_")
	if label == "" {
		id := strings.ReplaceAll(fallbackID, "-", "")
		if len(id) > 8 {
			id = id[:8]
		}
		if id == "" {
			return "ISO"
		}
		return "ISO-" + id
	}
	if len(label) > 64 {
		label = strings.TrimSpace(label[:64])
	}
	return label
}

// ItemID returns a safe iPXE label id.
func ItemID(prefix, raw string) string {
	raw = strings.ReplaceAll(raw, "-", "")
	raw = MenuText(raw)
	raw = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return -1
	}, raw)
	if len(raw) > 16 {
		raw = raw[:16]
	}
	if raw == "" {
		raw = "item"
	}
	return prefix + raw
}
