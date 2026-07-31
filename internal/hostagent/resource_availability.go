package hostagent

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const resourceAvailabilityTTL = 15 * time.Second

type resourceAvailability struct {
	ResourceID string    `json:"resource_id"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func (s *Server) getCapabilityAvailability(writer http.ResponseWriter, request *http.Request) {
	userID, resourceID := localUserID(request), chi.URLParam(request, "resourceID")
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	now := time.Now().UTC()
	cacheKey := userID + "\x00" + resourceID
	s.availabilityMu.Lock()
	if cached, ok := s.availability[cacheKey]; ok && now.Before(cached.ExpiresAt) {
		s.availabilityMu.Unlock()
		writeJSON(writer, http.StatusOK, cached)
		return
	}
	s.availabilityMu.Unlock()
	s.mu.RLock()
	user := s.config.Users[userID]
	resource, found := LocalCapabilityResource{}, false
	if user != nil {
		resource, found = user.Capabilities[resourceID]
	}
	s.mu.RUnlock()
	if !found {
		writeError(writer, http.StatusNotFound, "capability resource not found")
		return
	}
	manifest, known := adapterManifest(resource.AdapterID)
	status := resourceAvailability{
		ResourceID: resourceID, Status: "unavailable",
		CheckedAt: now, ExpiresAt: now.Add(resourceAvailabilityTTL),
	}
	if !known {
		status.Reason = "adapter is unavailable"
	} else if path, err := s.lookPath(manifest.Executable); err != nil || strings.TrimSpace(path) == "" {
		status.Reason = "required CLI is not installed"
	} else {
		status.Status = "available"
	}
	s.availabilityMu.Lock()
	s.availability[cacheKey] = status
	s.availabilityMu.Unlock()
	writeJSON(writer, http.StatusOK, status)
}
