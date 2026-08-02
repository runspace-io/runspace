package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	"github.com/runspace/runspace/internal/agent"
	"github.com/runspace/runspace/internal/agentregistry"
	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/events"
	"github.com/runspace/runspace/internal/filesync"
	"github.com/runspace/runspace/internal/git"
	"github.com/runspace/runspace/internal/observability"
	"github.com/runspace/runspace/internal/persistence"
	"github.com/runspace/runspace/internal/publish"
	"github.com/runspace/runspace/internal/realtime"
	repositoryapp "github.com/runspace/runspace/internal/repository"
	"github.com/runspace/runspace/internal/resourcegraph"
	"github.com/runspace/runspace/internal/runs"
	"github.com/runspace/runspace/internal/runtime"
	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/secrets"
	"github.com/runspace/runspace/internal/terminal"
	"github.com/runspace/runspace/internal/workspace"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	databaseCleanup, databaseStore, err := initializeDatabase(os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Error("initialize PostgreSQL", "error", err)
		return
	}
	defer databaseCleanup()
	router := chi.NewRouter()
	router.Use(localCORS)
	metrics := observability.New()
	router.Use(metrics.Middleware)
	workspaceService := workspace.NewMemoryService(time.Now)
	if databaseStore != nil {
		workspaceService.SetStore(databaseStore)
	}
	hub := realtime.NewMemoryHub()
	publisher, subscription, err := initializeNATS(os.Getenv("NATS_URL"), hub)
	if err != nil {
		logger.Error("initialize NATS", "error", err)
		return
	}
	defer closeNATS(publisher, subscription)
	chatService := newChatService(workspaceService, databaseStore, publisher)
	api, err := buildAPI(workspaceService, chatService, hub, publisher, databaseStore)
	if err != nil {
		logger.Error("initialize API", "error", err)
		return
	}
	signer, err := auth.NewSigner(gatewayAuthSecret(), time.Now)
	if err != nil {
		logger.Error("initialize gateway authentication", "error", err)
		return
	}
	// Everything under /api/v1 requires a verified token. /healthz and /metrics
	// stay open so orchestrators can probe without a credential.
	router.With(auth.Middleware(signer)).Mount("/api/v1", api)
	router.Get("/healthz", healthHandler(logger))
	router.Get("/metrics", metrics.Handler)
	serve(logger, router)
}

func localCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			writer.Header().Set("Vary", "Origin")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func buildAPI(workspaceService *workspace.MemoryService, chatService *collaboration.MemoryService, hub *realtime.MemoryHub, publisher *events.NATSPublisher, databaseStore *persistence.Store) (chi.Router, error) {
	api := chi.NewRouter()
	graph := resourcegraph.New(workspaceService, time.Now)
	if databaseStore != nil {
		graph.SetStore(databaseStore)
	}
	workspaceService.SetGraphProjector(graph)
	chatService.SetGraphProjector(graph)
	workspace.NewHandler(workspaceService).RegisterRoutes(api)
	collaboration.NewHandler(chatService).RegisterRoutes(api)
	agentRegistry, hostAgentURL := newAgentRegistry(
		workspaceService, chatService, graph, databaseStore, publisher,
	)
	agentregistry.NewHandler(agentRegistry).RegisterRoutes(api)
	graphHandler := resourcegraph.NewHandler(graph, chatService)
	graphHandler.SetAgentMessageWriter(agentRegistry)
	if err := configureResourcePlugins(
		api, graphHandler, workspaceService, graph, databaseStore, hostAgentURL,
	); err != nil {
		return nil, err
	}
	graphHandler.RegisterRoutes(api)
	secretStore, err := secrets.New(chatService, workspaceService, channelSecretKey(), time.Now)
	if err != nil {
		return nil, err
	}
	if databaseStore != nil {
		secretStore.SetPersistence(databaseStore)
	}
	secrets.NewHandler(secretStore).RegisterRoutes(api)
	repositoryRoot := os.Getenv("REPOSITORY_ROOT")
	if repositoryRoot == "" {
		repositoryRoot = "/var/lib/runspace/repositories"
	}
	resolver, err := sandbox.NewLayoutResolver(repositoryRoot)
	if err != nil {
		return nil, err
	}
	sandboxService, err := sandbox.NewService(resolver, sandbox.Config{})
	if err != nil {
		return nil, err
	}
	sandbox.NewHandler(sandboxService, workspaceService, workspaceService, git.NewProvider()).RegisterRoutes(api)
	repositoryService, err := newRepositoryService(workspaceService, resolver, publisher)
	if err != nil {
		return nil, err
	}
	repositoryapp.NewHandler(repositoryService).RegisterRoutes(api)
	if err := registerFileSyncRoutes(api, workspaceService, resolver); err != nil {
		return nil, err
	}
	terminal.NewHandler(terminal.NewDockerFactoryWithVolume(os.Getenv("AGENT_IMAGE"), os.Getenv("REPOSITORY_VOLUME")), resolver, workspaceService).RegisterRoutes(api)
	defaultAgent := newAgentRuntime()
	runService := runs.New(defaultAgent, publisher)
	if databaseStore != nil {
		runService.SetStore(databaseStore)
	}
	runService.SetAgentFactory(func(command string) contracts.Agent {
		if command != "" {
			return runtime.NewACP(runtime.NewStdioACPFactory(command))
		}
		return defaultAgent
	})
	runs.NewHandler(runService, workspaceService, chatService, resolver).RegisterRoutes(api)
	remote := git.GitHubRemote{BaseURL: os.Getenv("GITHUB_API_URL"), Token: os.Getenv("GITHUB_TOKEN")}
	publish.NewHandler(publish.New(git.NewProvider(), remote), workspaceService, workspaceService, resolver).RegisterRoutes(api)
	api.Handle("/realtime", realtime.NewHandler(hub, workspaceService, publisher))
	return api, nil
}

func registerFileSyncRoutes(api chi.Router, workspaces *workspace.MemoryService, resolver sandbox.RootResolver) error {
	syncthingURL := strings.TrimSpace(os.Getenv("SYNCTHING_URL"))
	syncthingAPIKey := strings.TrimSpace(os.Getenv("SYNCTHING_API_KEY"))
	if syncthingURL == "" && syncthingAPIKey == "" {
		return nil
	}
	engine, err := filesync.NewSyncthingClient(syncthingURL, syncthingAPIKey)
	if err != nil {
		return fmt.Errorf("configure file sync: %w", err)
	}
	addresses := strings.Split(os.Getenv("SYNCTHING_GATEWAY_ADDRESSES"), ",")
	service, err := filesync.NewService(workspaces, resolver, engine, addresses)
	if err != nil {
		return fmt.Errorf("initialize file sync: %w", err)
	}
	filesync.NewHandler(service).RegisterRoutes(api)
	return nil
}

func newAgentRuntime() contracts.Agent {
	command := os.Getenv("ACP_COMMAND")
	if command != "" {
		return runtime.NewACP(runtime.NewStdioACPFactory(command))
	}
	return agent.NewMockRuntime()
}

func newRepositoryService(workspaces *workspace.MemoryService, resolver sandbox.RootResolver, publisher *events.NATSPublisher) (*repositoryapp.Service, error) {
	if publisher == nil {
		return repositoryapp.NewService(workspaces, resolver, git.NewProvider())
	}
	return repositoryapp.NewServiceWithPublisher(workspaces, resolver, git.NewProvider(), publisher)
}

func healthHandler(logger *slog.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(map[string]string{
			"status": "ok", "time": time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			logger.ErrorContext(request.Context(), "write health response", "error", err)
		}
	}
}

func serve(logger *slog.Logger, router http.Handler) {
	runServer(logger, router)
}

func runServer(logger *slog.Logger, router http.Handler) {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      130 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("gateway listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("gateway stopped", "error", err)
	}
}

func newChatService(authorizer *workspace.MemoryService, store *persistence.Store, publisher *events.NATSPublisher) *collaboration.MemoryService {
	service := collaboration.NewMemoryService(time.Now, authorizer)
	if store != nil {
		service.SetStore(store)
	}
	if publisher != nil {
		service.SetPublisher(publisher)
	}
	return service
}

func closeNATS(publisher *events.NATSPublisher, subscription *nats.Subscription) {
	if publisher == nil {
		return
	}
	_ = subscription.Unsubscribe()
	publisher.Close()
}

func initializeDatabase(databaseURL string) (func(), *persistence.Store, error) {
	if databaseURL == "" {
		return func() {}, nil, nil
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return func() {}, nil, fmt.Errorf("open database: %w", err)
	}
	cleanup := func() { _ = database.Close() }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store := persistence.New(database)
	if err := store.Ping(ctx); err != nil {
		cleanup()
		return func() {}, nil, fmt.Errorf("ping database: %w", err)
	}
	if err := store.Migrate(ctx); err != nil {
		cleanup()
		return func() {}, nil, fmt.Errorf("migrate database: %w", err)
	}
	return cleanup, store, nil
}

func initializeNATS(url string, hub *realtime.MemoryHub) (*events.NATSPublisher, *nats.Subscription, error) {
	if url == "" {
		return nil, nil, nil
	}
	publisher, err := events.ConnectNATS(url)
	if err != nil {
		return nil, nil, err
	}
	subscription, err := publisher.Subscribe("gateway-realtime", "evt.>", hub.Publish)
	if err != nil {
		publisher.Close()
		return nil, nil, err
	}
	return publisher, subscription, nil
}
