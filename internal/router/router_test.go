package router

import (
	"net/http"
	"testing"
	"testing/fstest"
)

func TestEmbedFileSystemExistsAcceptsLeadingSlash(t *testing.T) {
	fileSystem := embedFileSystem{
		FileSystem: http.FS(fstest.MapFS{
			"assets/app.js":           {Data: []byte("console.log('ok')")},
			"provider-icons/chat.svg": {Data: []byte("<svg></svg>")},
		}),
	}

	tests := []string{
		"/assets/app.js",
		"assets/app.js",
		"/provider-icons/chat.svg",
		"provider-icons/chat.svg",
		"/",
	}

	for _, path := range tests {
		if !fileSystem.Exists("/", path) {
			t.Fatalf("expected %q to exist", path)
		}
	}
}
