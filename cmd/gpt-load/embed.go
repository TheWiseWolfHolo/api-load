package main

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed all:dist
var content embed.FS

// WebUI is the filesystem for the embedded web UI.
var WebUI fs.FS

func init() {
	var err error
	WebUI, err = fs.Sub(content, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem for UI: %v", err)
	}
}