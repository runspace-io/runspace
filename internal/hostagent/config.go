package hostagent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/workspace"
)

const localConfigVersion = 1

type LocalConfig struct {
	Version  int                         `json:"version"`
	DeviceID string                      `json:"device_id"`
	Users    map[string]*LocalUserConfig `json:"users"`
}

type LocalUserConfig struct {
	Resources    map[string]LocalResourceBinding    `json:"resources"`
	Capabilities map[string]LocalCapabilityResource `json:"capability_resources"`
	Agents       map[string]LocalAgentPreference    `json:"agents"`
	Sessions     map[string]LocalACPSession          `json:"acp_sessions"`
}

type LocalResourceBinding struct {
	Path        string             `json:"path"`
	GatewayURL  string             `json:"gateway_url,omitempty"`
	WorkspaceID string             `json:"workspace_id"`
	Resource    workspace.Resource `json:"resource"`
}

type LocalAgentPreference struct {
	Model          string `json:"model,omitempty"`
	PermissionMode string `json:"permission_mode"`
}

type LocalACPSession struct {
	Title      string                `json:"title,omitempty"`
	AgentID    string                `json:"agent_id"`
	ResourceID string                `json:"resource_id"`
	ThreadID   string                `json:"thread_id"`
	SessionID  string                `json:"session_id"`
	Status     string                `json:"status,omitempty"`
	Messages   []LocalSessionMessage `json:"messages,omitempty"`
	UpdatedAt  string                `json:"updated_at,omitempty"`
}

type LocalSessionMessage struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) EnableApprovalPersistence() error {
	stateDir, err := syncStateDir()
	if err != nil {
		return err
	}
	s.configFile = filepath.Join(stateDir, "runspace-local.json")
	payload, err := os.ReadFile(s.configFile)
	if os.IsNotExist(err) {
		s.config = migratedLocalConfig(stateDir)
		return s.saveConfig()
	}
	if err != nil {
		return err
	}
	var config LocalConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return err
	}
	if config.Version != localConfigVersion || strings.TrimSpace(config.DeviceID) == "" {
		return errors.New("unsupported local config version")
	}
	normalizeLocalConfig(&config)
	if len(config.Users) == 0 {
		config = migrateIntoEmptyConfig(config, stateDir)
	}
	s.mu.Lock()
	s.config = config
	s.deviceID = config.DeviceID
	s.rebuildMirrorIndexLocked()
	s.mu.Unlock()
	return s.saveConfig()
}

func migratedLocalConfig(stateDir string) LocalConfig {
	deviceID := ephemeralDeviceID()
	if payload, err := os.ReadFile(filepath.Join(stateDir, "device-id")); err == nil &&
		strings.TrimSpace(string(payload)) != "" {
		deviceID = strings.TrimSpace(string(payload))
	}
	return migrateIntoEmptyConfig(newLocalConfig(deviceID), stateDir)
}

func migrateIntoEmptyConfig(config LocalConfig, stateDir string) LocalConfig {
	if device, err := os.ReadFile(filepath.Join(stateDir, "device-id")); err == nil &&
		strings.TrimSpace(string(device)) != "" {
		config.DeviceID = strings.TrimSpace(string(device))
	}
	payload, err := os.ReadFile(filepath.Join(stateDir, "approved-folders.json"))
	if err != nil {
		return config
	}
	legacy := map[string]string{}
	if json.Unmarshal(payload, &legacy) != nil || len(legacy) == 0 {
		return config
	}
	admin := &LocalUserConfig{
		Resources:    map[string]LocalResourceBinding{},
		Capabilities: map[string]LocalCapabilityResource{},
		Agents:       map[string]LocalAgentPreference{},
		Sessions:     map[string]LocalACPSession{},
	}
	for resourceID, path := range legacy {
		admin.Resources[resourceID] = LocalResourceBinding{
			Path: path, Resource: workspace.Resource{ID: resourceID},
		}
	}
	config.Users["admin"] = admin
	return config
}

func newLocalConfig(deviceID string) LocalConfig {
	return LocalConfig{Version: localConfigVersion, DeviceID: deviceID, Users: map[string]*LocalUserConfig{}}
}

func normalizeLocalConfig(config *LocalConfig) {
	if config.Users == nil {
		config.Users = map[string]*LocalUserConfig{}
	}
	for _, user := range config.Users {
		if user.Resources == nil {
			user.Resources = map[string]LocalResourceBinding{}
		}
		if user.Agents == nil {
			user.Agents = map[string]LocalAgentPreference{}
		}
		if user.Capabilities == nil {
			user.Capabilities = map[string]LocalCapabilityResource{}
		}
		if user.Sessions == nil {
			user.Sessions = map[string]LocalACPSession{}
		}
	}
}

func (s *Server) userConfigLocked(userID string) *LocalUserConfig {
	user := s.config.Users[userID]
	if user == nil {
		user = &LocalUserConfig{
			Resources:    map[string]LocalResourceBinding{},
			Capabilities: map[string]LocalCapabilityResource{},
			Agents:       map[string]LocalAgentPreference{},
			Sessions:     map[string]LocalACPSession{},
		}
		s.config.Users[userID] = user
	}
	return user
}

func (s *Server) approveMirror(userID string, binding LocalResourceBinding) error {
	s.mu.Lock()
	user := s.userConfigLocked(userID)
	user.Resources[binding.Resource.ID] = binding
	s.mirrors[scopedResourceKey(userID, binding.Resource.ID)] = binding.Path
	s.mu.Unlock()
	return s.saveConfig()
}

func (s *Server) saveConfig() error {
	s.mu.RLock()
	payload, err := json.MarshalIndent(s.config, "", "  ")
	path := s.configFile
	s.mu.RUnlock()
	if err != nil || path == "" {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

func (s *Server) rebuildMirrorIndexLocked() {
	s.mirrors = make(map[string]string)
	for userID, user := range s.config.Users {
		for resourceID, binding := range user.Resources {
			s.mirrors[scopedResourceKey(userID, resourceID)] = binding.Path
		}
	}
}

func (s *Server) exportConfig(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	s.mu.RLock()
	user := s.config.Users[userID]
	exported := LocalConfig{Version: localConfigVersion, DeviceID: s.deviceID, Users: map[string]*LocalUserConfig{}}
	if user != nil {
		copy := *user
		exported.Users[userID] = &copy
	} else {
		exported.Users[userID] = &LocalUserConfig{
			Resources:    map[string]LocalResourceBinding{},
			Capabilities: map[string]LocalCapabilityResource{},
			Agents:       map[string]LocalAgentPreference{},
			Sessions:     map[string]LocalACPSession{},
		}
	}
	s.mu.RUnlock()
	writer.Header().Set("Content-Disposition", `attachment; filename="runspace-local.json"`)
	writeJSON(writer, http.StatusOK, exported)
}

func (s *Server) importConfig(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	var imported LocalConfig
	if err := decodeRequest(writer, request, &imported); err != nil ||
		imported.Version != localConfigVersion || imported.Users[userID] == nil {
		writeError(writer, http.StatusBadRequest, "invalid or mismatched local config")
		return
	}
	normalizeLocalConfig(&imported)
	s.mu.Lock()
	s.config.Users[userID] = imported.Users[userID]
	s.rebuildMirrorIndexLocked()
	s.mu.Unlock()
	if err := s.saveConfig(); err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "imported"})
}

func (s *Server) saveAgentPreference(writer http.ResponseWriter, request *http.Request) {
	userID := localUserID(request)
	agentID := chi.URLParam(request, "agentID")
	if userID == "" {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	var preference LocalAgentPreference
	if decodeRequest(writer, request, &preference) != nil ||
		(preference.PermissionMode != "default" && preference.PermissionMode != "approve" &&
			preference.PermissionMode != "yolo") {
		writeError(writer, http.StatusBadRequest, "invalid agent preference")
		return
	}
	s.mu.Lock()
	s.userConfigLocked(userID).Agents[agentID] = preference
	s.mu.Unlock()
	if err := s.saveConfig(); err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, preference)
}

func localUserID(request *http.Request) string {
	return strings.TrimSpace(fallback(request.Header.Get("X-User-ID"), request.URL.Query().Get("user_id")))
}

func scopedResourceKey(userID, resourceID string) string { return userID + "\x00" + resourceID }

func ephemeralDeviceID() string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "ephemeral"
	}
	return hex.EncodeToString(random)
}
