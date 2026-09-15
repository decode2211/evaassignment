// This tiny file's only job is to bundle index.html directly into the
// compiled server binary using Go's "embed" feature. That means the
// finished program is a single file (or a single Docker image layer) with
// no separate frontend assets to copy around or lose track of.
package web

import "embed"

// FS holds the embedded frontend files. cmd/server/main.go serves this at
// GET / via http.FileServer.
//
//go:embed index.html
var FS embed.FS
