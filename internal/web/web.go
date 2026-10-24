// Package web embeds the single-page demo UI for the freight class recommender.
package web

import _ "embed"

// IndexHTML is the self-contained demo page served at the server root.
//
//go:embed index.html
var IndexHTML []byte
