package api

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed dashboard
var dashboardFS embed.FS

// RegisterDashboard mounts the embedded test dashboard at the root path.
func RegisterDashboard(mux *http.ServeMux) {
	// Strip the "dashboard" prefix so index.html is served at /
	sub, err := fs.Sub(dashboardFS, "dashboard")
	if err != nil {
		log.Fatalf("failed to load embedded dashboard: %v", err)
	}

	fileServer := http.FileServer(http.FS(sub))

	// Serve index.html at root
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only serve dashboard for root path, let other handlers take priority
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
}
