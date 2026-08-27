package web

import "embed"

// Static holds the admin UI files.
//
//go:embed index.html app.js style.css
var Static embed.FS
