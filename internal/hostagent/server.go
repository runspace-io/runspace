package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/filesync"
	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/workspace"
)

type Server struct {
	engine         filesync.Engine
	httpClient     *http.Client
	gitBinary      string
	mu             sync.RWMutex
	mirrors        map[string]string
	config         LocalConfig
	configFile     string
	browser        *sandbox.Service
	deviceID       string
	lookPath       func(string) (string, error)
	agentMu        sync.RWMutex
	agentLaunch    map[string]agentLaunch
	sessionMu      sync.Mutex
	sessions       map[string]*agentSession
	newACPClient   agentClientFactory
	availabilityMu sync.Mutex
	availability   map[string]resourceAvailability
}

type RepositoryStatus struct {
	Path       string `json:"path"`
	Git        bool   `json:"git"`
	Origin     string `json:"origin,omitempty"`
	Branch     string `json:"branch,omitempty"`
	HasRemote  bool   `json:"has_remote"`
	CanConnect bool   `json:"can_connect"`
}

func NewServer(engine filesync.Engine) (*Server, error) {
	server := &Server{
		engine:       engine,
		httpClient:   &http.Client{Timeout: 2 * time.Minute},
		gitBinary:    "git",
		mirrors:      make(map[string]string),
		deviceID:     ephemeralDeviceID(),
		lookPath:     exec.LookPath,
		agentLaunch:  make(map[string]agentLaunch),
		sessions:     make(map[string]*agentSession),
		newACPClient: defaultAgentClient,
		availability: make(map[string]resourceAvailability),
	}
	server.config = newLocalConfig(server.deviceID)
	browser, err := sandbox.NewService(approvedMirrorResolver{server: server}, sandbox.Config{})
	if err != nil {
		return nil, err
	}
	server.browser = browser
	return server, nil
}

func (s *Server) Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(loopbackCORS)
	router.Get("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/v1/status", s.status)
	router.Get("/v1/agents", s.discoverAgents)
	router.Get("/v1/agents/discover", s.discoverAgents)
	router.Get("/v1/agent-chats", s.listAgentChats)
	router.Put("/v1/agents/{agentID}/preferences", s.saveAgentPreference)
	router.Post("/v1/agents/{agentID}/prompt", s.promptAgent)
	router.Get("/v1/agents/{agentID}/session", s.getAgentSession)
	router.Post("/v1/agents/{agentID}/session/cancel", s.cancelAgentSession)
	router.Get("/v1/agents/health", s.agentHealth)
	router.Get("/v1/agents/{agentID}/models", s.agentModels)
	router.Get("/v1/config/export", s.exportConfig)
	router.Post("/v1/config/import", s.importConfig)
	router.Get("/v1/resource-adapters", s.discoverAdapters)
	router.Get("/v1/capability-resources", s.listCapabilityResources)
	router.Post("/v1/capability-resources", s.connectCapabilityResource)
	router.Post("/v1/capability-resources/{resourceID}/query", s.queryCapabilityResource)
	router.Get("/v1/capability-resources/{resourceID}/availability", s.getCapabilityAvailability)
	router.Post("/v1/filesystem/suggest", s.suggestDirectories)
	router.Post("/v1/resources/inspect", s.inspectRepository)
	router.Post("/v1/resources/init", s.initializeRepository)
	router.Post("/v1/resources", s.createMirror)
	router.Get("/v1/resources/{repositoryID}/tree", s.repositoryTree)
	router.Get("/v1/resources/{repositoryID}/file", s.repositoryFile)
	router.Get("/v1/resources/{repositoryID}/terminal", s.openTerminal)
	// Deprecated aliases retained for already-installed web clients.
	router.Post("/v1/repositories/inspect", s.inspectRepository)
	router.Post("/v1/repositories/init", s.initializeRepository)
	router.Post("/v1/mirrors", s.createMirror)
	router.Get("/v1/repositories/{repositoryID}/tree", s.repositoryTree)
	router.Get("/v1/repositories/{repositoryID}/file", s.repositoryFile)
	router.Get("/v1/terminals/{repositoryID}", s.openTerminal)
	return router
}

func (s *Server) createMirror(writer http.ResponseWriter, request *http.Request) {
	var body MirrorRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid mirror request")
		return
	}
	result, err := s.CreateMirror(request.Context(), body)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, result)
}

func (s *Server) CreateMirror(ctx context.Context, request MirrorRequest) (MirrorResponse, error) {
	path, gatewayURL, err := validateRequest(request)
	if err != nil {
		return MirrorResponse{}, err
	}
	status, err := s.InspectRepository(ctx, path)
	if err != nil {
		return MirrorResponse{}, err
	}
	origin, branch := status.Origin, status.Branch

	repository := workspace.Repository{}
	cloneURL := origin
	if cloneURL == "" {
		cloneURL = s.localResourceURL(path)
	}
	provider := "folder"
	if status.Git {
		provider = "mirror"
	}
	if err := s.gateway(ctx, gatewayURL+"/workspaces/"+url.PathEscape(request.WorkspaceID)+"/resources", request.UserID, map[string]string{
		"provider":       provider,
		"full_name":      repositoryName(origin, path),
		"clone_url":      cloneURL,
		"default_branch": branch,
	}, &repository); err != nil {
		return MirrorResponse{}, fmt.Errorf("register resource: %w", err)
	}
	if s.engine == nil {
		if err := s.approveMirror(request.UserID, LocalResourceBinding{
			Path: path, GatewayURL: gatewayURL,
			WorkspaceID: request.WorkspaceID, Resource: repository,
		}); err != nil {
			return MirrorResponse{}, fmt.Errorf("save host approval: %w", err)
		}
		return MirrorResponse{
			Resource:   repository,
			Repository: repository,
			Sync: filesync.Session{
				ID:           "lazy:" + repository.ID,
				WorkspaceID:  request.WorkspaceID,
				RepositoryID: repository.ID,
				Status:       filesync.FolderStatus{State: "lazy"},
				CreatedAt:    time.Now().UTC(),
			},
		}, nil
	}
	if origin != "" {
		seedURL := gatewayURL + "/workspaces/" + url.PathEscape(request.WorkspaceID) + "/resources/" + url.PathEscape(repository.ID) + "/clone"
		if err := s.gateway(ctx, seedURL, request.UserID, struct{}{}, nil); err != nil {
			return MirrorResponse{}, fmt.Errorf("seed container checkout: %w", err)
		}
	}

	hostDeviceID, err := s.engine.DeviceID(ctx)
	if err != nil {
		return MirrorResponse{}, fmt.Errorf("read local sync device: %w", err)
	}
	hostname, _ := os.Hostname()
	syncURL := gatewayURL + "/workspaces/" + url.PathEscape(request.WorkspaceID) + "/resources/" + url.PathEscape(repository.ID) + "/sync"
	session := filesync.Session{}
	if err := s.gateway(ctx, syncURL, request.UserID, map[string]any{
		"device_id":   hostDeviceID,
		"device_name": fallback(strings.TrimSpace(hostname), "Runspace host"),
		"addresses":   []string{"tcp://host.docker.internal:22000"},
		"branch":      branch,
		"git":         status.Git,
	}, &session); err != nil {
		return MirrorResponse{}, fmt.Errorf("pair gateway sync device: %w", err)
	}
	if err := s.engine.UpsertDevice(ctx, filesync.Device{
		ID:        session.GatewayDeviceID,
		Name:      "Runspace gateway",
		Addresses: session.GatewayAddresses,
	}); err != nil {
		return MirrorResponse{}, fmt.Errorf("pair local sync device: %w", err)
	}
	if err := s.engine.UpsertFolder(ctx, filesync.Folder{
		ID:               session.FolderID,
		Label:            repository.FullName,
		Path:             path,
		Type:             "sendreceive",
		FSWatcherEnabled: true,
		FSWatcherDelayS:  1,
		IgnorePerms:      true,
		Devices:          []filesync.FolderDevice{{DeviceID: session.GatewayDeviceID}},
	}); err != nil {
		return MirrorResponse{}, fmt.Errorf("configure local mirror: %w", err)
	}
	if err := s.engine.SetIgnores(ctx, session.FolderID, filesync.MirrorIgnorePatterns()); err != nil {
		return MirrorResponse{}, fmt.Errorf("protect Git metadata: %w", err)
	}
	if err := s.approveMirror(request.UserID, LocalResourceBinding{
		Path: path, GatewayURL: gatewayURL,
		WorkspaceID: request.WorkspaceID, Resource: repository,
	}); err != nil {
		return MirrorResponse{}, fmt.Errorf("save terminal approval: %w", err)
	}
	return MirrorResponse{Resource: repository, Repository: repository, Sync: session}, nil
}

func loopbackCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			writer.Header().Set("Vary", "Origin")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}
