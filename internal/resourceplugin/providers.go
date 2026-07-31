package resourceplugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/runspace/runspace/internal/resourcegraph"
)

const maxProviderBody = 1 << 20

type providerClients struct {
	http         *http.Client
	githubURL    string
	digitalURL   string
	openPostgres func(string) (*sql.DB, error)
}

func defaultProviderClients() providerClients {
	return providerClients{
		http:      &http.Client{Timeout: 15 * time.Second},
		githubURL: "https://api.github.com", digitalURL: "https://api.digitalocean.com",
		openPostgres: func(dsn string) (*sql.DB, error) { return sql.Open("pgx", dsn) },
	}
}

func (s *Service) Query(
	ctx context.Context, ownerID, resourceID, placement string, input resourcegraph.CapabilityQuery,
) (any, error) {
	connection, err := s.connection(ctx, resourceID)
	if err != nil || connection.OwnerID != ownerID || placement != "runspace" {
		return nil, ErrNotFound
	}
	plugin, ok := manifest(connection.PluginID)
	if !ok || !hasCapability(plugin, input.Capability) || len(input.Query) > 240 {
		return nil, ErrInvalid
	}
	credential, err := openCredential(s.key, connection.Secret)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	matches, err := s.providerQuery(ctx, connection, credential, input.Capability, input.Query, limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"resource_id": resourceID, "capability": input.Capability,
		"matches": matches, "truncated": len(matches) == limit,
	}, nil
}

func (s *Service) Availability(
	ctx context.Context, ownerID, resourceID, placement string,
) (any, error) {
	if placement != "runspace" {
		return nil, ErrNotFound
	}
	now := s.now().UTC()
	s.mu.RLock()
	cached, ok := s.availability[resourceID]
	s.mu.RUnlock()
	if ok && now.Before(cached.ExpiresAt) {
		return cached, nil
	}
	connection, err := s.connection(ctx, resourceID)
	if err != nil || connection.OwnerID != ownerID {
		return nil, ErrNotFound
	}
	credential, err := openCredential(s.key, connection.Secret)
	status := Availability{
		ResourceID: resourceID, Status: "unavailable",
		CheckedAt: now, ExpiresAt: now.Add(availabilityTTL),
	}
	if err != nil {
		status.Reason = "Credential could not be opened."
	} else if err = s.providerHealth(ctx, connection, credential); err != nil {
		status.Reason = "Provider rejected the connection or is unreachable."
	} else {
		status.Status = "available"
	}
	s.mu.Lock()
	s.availability[resourceID] = status
	s.mu.Unlock()
	return status, nil
}

func (s *Service) providerHealth(
	ctx context.Context, connection Connection, credential string,
) error {
	switch connection.PluginID {
	case "github":
		return s.authorizedGET(ctx, s.providers.githubURL+"/user", credential, "github", nil)
	case "digitalocean":
		return s.authorizedGET(ctx, s.providers.digitalURL+"/v2/account", credential, "digitalocean", nil)
	case "postgresql":
		database, err := s.providers.openPostgres(credential)
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()
		return database.PingContext(ctx)
	default:
		return ErrInvalid
	}
}

func (s *Service) providerQuery(
	ctx context.Context, connection Connection, credential, capability, query string, limit int,
) ([]resourcegraph.CapabilityMatch, error) {
	switch capability {
	case "github.repositories.query":
		return s.githubRepositories(ctx, credential, query, limit)
	case "github.pull_requests.query":
		return s.githubPullRequests(ctx, credential, query, limit)
	case "digitalocean.apps.query":
		var payload struct {
			Apps []map[string]any `json:"apps"`
		}
		err := s.authorizedGET(ctx, s.providers.digitalURL+"/v2/apps?per_page=50", credential, "digitalocean", &payload)
		return filterMatches(mapRows(payload.Apps, digitalOceanApp), query, limit), err
	case "digitalocean.droplets.query":
		var payload struct {
			Droplets []map[string]any `json:"droplets"`
		}
		err := s.authorizedGET(ctx, s.providers.digitalURL+"/v2/droplets?per_page=50", credential, "digitalocean", &payload)
		return filterMatches(mapRows(payload.Droplets, digitalOceanDroplet), query, limit), err
	case "postgres.schema.query":
		return s.postgresSchema(ctx, credential, query, limit)
	default:
		return nil, ErrInvalid
	}
}

func (s *Service) githubRepositories(
	ctx context.Context, credential, query string, limit int,
) ([]resourcegraph.CapabilityMatch, error) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	target := s.providers.githubURL + "/search/repositories?q=" +
		url.QueryEscape(defaultText(query, "stars:>0")) + "&per_page=" + strconv.Itoa(limit)
	err := s.authorizedGET(ctx, target, credential, "github", &payload)
	return mapRows(payload.Items, githubRepository), err
}

func (s *Service) githubPullRequests(
	ctx context.Context, credential, query string, limit int,
) ([]resourcegraph.CapabilityMatch, error) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	target := s.providers.githubURL + "/search/issues?q=" +
		url.QueryEscape("is:pr "+defaultText(query, "is:open")) + "&per_page=" + strconv.Itoa(limit)
	err := s.authorizedGET(ctx, target, credential, "github", &payload)
	return mapRows(payload.Items, githubPullRequest), err
}

func (s *Service) authorizedGET(
	ctx context.Context, target, credential, provider string, output any,
) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Accept", "application/json")
	if provider == "github" {
		request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	response, err := s.providers.http.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxProviderBody))
		return fmt.Errorf("provider returned status %d", response.StatusCode)
	}
	if output == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(response.Body, maxProviderBody))
		return err
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxProviderBody))
	if decoder.Decode(output) != nil {
		return errors.New("provider returned invalid structured output")
	}
	return nil
}
