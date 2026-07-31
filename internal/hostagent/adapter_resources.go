package hostagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

var safeProfile = regexp.MustCompile(`^[A-Za-z0-9_.@/-]{0,120}$`)

type capabilityConnectRequest struct {
	AdapterID   string `json:"adapter_id"`
	Title       string `json:"title"`
	Profile     string `json:"profile"`
	GatewayURL  string `json:"gateway_url"`
	WorkspaceID string `json:"workspace_id"`
}

func (s *Server) listCapabilityResources(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	s.mu.RLock()
	items := make([]LocalCapabilityResource, 0)
	if user := s.config.Users[userID]; user != nil {
		for _, item := range user.Capabilities {
			item.Capabilities = append([]CapabilityDescriptor(nil), item.Capabilities...)
			items = append(items, item)
		}
	}
	s.mu.RUnlock()
	writeJSON(writer, http.StatusOK, map[string]any{"resources": items})
}

func (s *Server) connectCapabilityResource(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	var input capabilityConnectRequest
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	if decodeRequest(writer, request, &input) != nil {
		writeError(writer, http.StatusBadRequest, "invalid capability resource")
		return
	}
	resource, err := s.connectCapability(request.Context(), userID, input)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, resource)
}

func (s *Server) queryCapabilityResource(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	var input CapabilityQueryRequest
	if decodeRequest(writer, request, &input) != nil {
		writeError(writer, http.StatusBadRequest, "invalid resource query")
		return
	}
	result, err := s.queryCapability(
		request.Context(), userID, chi.URLParam(request, "resourceID"), input,
	)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (s *Server) connectCapability(
	ctx context.Context, userID string, input capabilityConnectRequest,
) (LocalCapabilityResource, error) {
	manifest, ok := adapterManifest(strings.TrimSpace(input.AdapterID))
	if !ok {
		return LocalCapabilityResource{}, errors.New("unsupported resource adapter")
	}
	if path, err := s.lookPath(manifest.Executable); err != nil || strings.TrimSpace(path) == "" {
		return LocalCapabilityResource{}, errors.New("required CLI is not installed")
	}
	title, profile := strings.TrimSpace(input.Title), strings.TrimSpace(input.Profile)
	if title == "" || len(title) > 120 || !safeProfile.MatchString(profile) ||
		(manifest.ID == "postgresql" && profile == "") {
		return LocalCapabilityResource{}, errors.New("safe title and profile are required")
	}
	gatewayURL, err := capabilityGatewayURL(input.GatewayURL)
	if err != nil || strings.TrimSpace(input.WorkspaceID) == "" {
		return LocalCapabilityResource{}, errors.New("valid gateway and workspace are required")
	}
	resource := LocalCapabilityResource{
		ID:        capabilityResourceID(s.deviceID, userID, manifest.ID, profile),
		AdapterID: manifest.ID, Title: title, Profile: profile,
		GatewayURL: gatewayURL, WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		Capabilities: append([]CapabilityDescriptor(nil), manifest.Capabilities...),
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.gateway(
		ctx, gatewayURL+"/workspaces/"+url.PathEscape(resource.WorkspaceID)+"/graph/nodes",
		userID, capabilityGraphNode(resource, manifest, userID), nil,
	); err != nil {
		return LocalCapabilityResource{}, err
	}
	s.mu.Lock()
	s.userConfigLocked(userID).Capabilities[resource.ID] = resource
	s.mu.Unlock()
	if err := s.saveConfig(); err != nil {
		return LocalCapabilityResource{}, err
	}
	return resource, nil
}

func capabilityGraphNode(
	resource LocalCapabilityResource, manifest AdapterManifest, ownerID string,
) map[string]any {
	return map[string]any{
		"id": "resource:" + resource.ID, "kind": "resource",
		"type": manifest.ID, "title": resource.Title, "owner_id": ownerID,
		"summary": manifest.Description,
		"metadata": map[string]any{
			"entity_id": resource.ID, "adapter_id": manifest.ID,
			"resource_type": manifest.ResourceType, "capabilities": resource.Capabilities,
			"placement": "host",
		},
	}
}

func capabilityGatewayURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(value), "/"))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("invalid gateway")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func capabilityResourceID(deviceID, userID, adapterID, profile string) string {
	sum := sha256.Sum256([]byte(deviceID + "\x00" + userID + "\x00" + adapterID + "\x00" + profile))
	return "local_capability_" + hex.EncodeToString(sum[:12])
}
