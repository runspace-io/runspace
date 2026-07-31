package hostagent

import (
	"net/http"
	"strings"
)

func (s *Server) discoverAdapters(writer http.ResponseWriter, request *http.Request) {
	if localUserID(request) == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	discovered := make([]AdapterDiscovery, 0, len(builtinAdapterManifests))
	for _, manifest := range builtinAdapterManifests {
		item := AdapterDiscovery{Manifest: manifest, Status: "not_installed"}
		if path, err := s.lookPath(manifest.Executable); err == nil && strings.TrimSpace(path) != "" {
			item.Status, item.Path = "ready", path
		}
		discovered = append(discovered, item)
	}
	writeJSON(writer, http.StatusOK, map[string]any{"adapters": discovered})
}
