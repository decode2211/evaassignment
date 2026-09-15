// Embeds the frontend into the compiled server binary.
package web

import "embed"

// FS holds the embedded frontend files, served at GET / by main.go.
//
//go:embed index.html
var FS embed.FS
