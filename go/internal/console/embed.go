/**
 * Author: Deepankar Das
 */

package console

import (
	"embed"
	"io/fs"
	"net/http"
)

// ConsoleFS holds both embedded Next.js static exports:
// - out-hub      (Hub Console)
// - out-sentinel (Sentinel Console)
//
//go:embed all:out-hub all:out-sentinel
var ConsoleFS embed.FS

// HubHandler serves the embedded Hub Console static app.
func HubHandler() http.Handler {
	return handlerForSubdir("out-hub", "Hub")
}

// SentinelHandler serves the embedded Sentinel Console static app.
func SentinelHandler() http.Handler {
	return handlerForSubdir("out-sentinel", "Sentinel")
}

func handlerForSubdir(subdir, label string) http.Handler {
	sub, err := fs.Sub(ConsoleFS, subdir)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(200)
			w.Write([]byte(`<!DOCTYPE html><html><body><h1>Enforcer ` + label + ` Console</h1><p>Console assets not embedded. Run 'make console' first.</p></body></html>`))
		})
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the exact file first.
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the file exists in the embedded FS.
		tryPath := path
		if tryPath[0] == '/' {
			tryPath = tryPath[1:]
		}
		if _, err := fs.Stat(sub, tryPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Try with index.html appended (for directory routes).
		if _, err := fs.Stat(sub, tryPath+"index.html"); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve the root index.html for client-side routing.
		indexData, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		w.Write(indexData)
	})
}
