// Package web embeds the frontend SPA build output.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"io/fs"
	"regexp"
)

//go:embed all:dist
var distFS embed.FS

var scriptTagRe = regexp.MustCompile(`<script[^>]*>([\s\S]*?)</script>`)

// Frontend returns the frontend filesystem rooted at dist/.
// Returns nil if the dist directory is empty (dev mode).
func Frontend() fs.FS {
	entries, err := fs.ReadDir(distFS, "dist")
	if err != nil || len(entries) == 0 {
		return nil
	}

	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}

	return sub
}

// InlineScriptHashes computes SHA-256 hashes of all inline scripts
// found in the embedded index.html. Returns nil if no frontend is embedded.
func InlineScriptHashes() []string {
	index, err := fs.ReadFile(distFS, "dist/index.html")
	if err != nil {
		return nil
	}

	matches := scriptTagRe.FindAllSubmatch(index, -1)

	var hashes []string

	for _, m := range matches {
		body := m[1]
		if len(body) == 0 {
			continue
		}

		h := sha256.Sum256(body)
		hashes = append(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(h[:])+"'")
	}

	return hashes
}
