// Package web embeds the compiled frontend (web/dist) into the Go binary so the
// admin console is served directly by MCPGW — no separate static file server or
// dev proxy needed for end users.
package web

import "embed"

// DistFS holds the built Vue frontend output, produced by `npm run build`
// (which writes to web/dist). It is served by the admin HTTP server.
//
//go:embed all:dist
var DistFS embed.FS
