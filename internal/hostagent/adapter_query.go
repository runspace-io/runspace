package hostagent

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const maxAdapterOutput = 1 << 20

func (s *Server) queryCapability(
	ctx context.Context, userID, resourceID string, input CapabilityQueryRequest,
) (CapabilityQueryResult, error) {
	s.mu.RLock()
	user := s.config.Users[userID]
	resource, found := LocalCapabilityResource{}, false
	if user != nil {
		resource, found = user.Capabilities[resourceID]
	}
	s.mu.RUnlock()
	if !found {
		return CapabilityQueryResult{}, errors.New("capability resource not found")
	}
	manifest, ok := adapterManifest(resource.AdapterID)
	if !ok || !capabilityAllowed(manifest, input.Capability) {
		return CapabilityQueryResult{}, errors.New("capability is not allowed")
	}
	query := strings.TrimSpace(input.Query)
	if len(query) > 240 {
		return CapabilityQueryResult{}, errors.New("query is too long")
	}
	limit := input.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	commandCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	matches, err := s.runAdapterQuery(commandCtx, manifest, resource, input.Capability, query, limit)
	if err != nil {
		return CapabilityQueryResult{}, err
	}
	return CapabilityQueryResult{
		ResourceID: resource.ID, Capability: input.Capability,
		Matches: matches, Truncated: len(matches) == limit,
	}, nil
}

func (s *Server) runAdapterQuery(
	ctx context.Context, manifest AdapterManifest, resource LocalCapabilityResource,
	capability, query string, limit int,
) ([]CapabilityMatch, error) {
	path, err := s.lookPath(manifest.Executable)
	if err != nil {
		return nil, errors.New("required CLI is unavailable")
	}
	switch capability {
	case "github.repositories.query":
		return jsonCommandMatches(ctx, path, []string{
			"search", "repos", fallback(query, "stars:>0"), "--limit", strconv.Itoa(limit),
			"--json", "fullName,description,url,visibility,updatedAt",
		}, "", limit, githubRepositoryMatch)
	case "github.pull_requests.query":
		return jsonCommandMatches(ctx, path, []string{
			"search", "prs", fallback(query, "is:open"), "--limit", strconv.Itoa(limit),
			"--json", "title,url,state,number,updatedAt,repository",
		}, "", limit, githubPullRequestMatch)
	case "digitalocean.apps.query":
		return jsonCommandMatches(
			ctx, path, []string{"apps", "list", "--output", "json"},
			query, limit, digitalOceanAppMatch,
		)
	case "digitalocean.droplets.query":
		return jsonCommandMatches(
			ctx, path, []string{"compute", "droplet", "list", "--output", "json"},
			query, limit, digitalOceanDropletMatch,
		)
	case "postgres.schema.query":
		return postgresSchemaMatches(ctx, path, resource.Profile, query, limit)
	default:
		return nil, errors.New("unsupported query capability")
	}
}

type matchMapper func(map[string]any) CapabilityMatch

func jsonCommandMatches(
	ctx context.Context, executable string, args []string, query string, limit int, mapper matchMapper,
) ([]CapabilityMatch, error) {
	output, err := boundedCommandOutput(ctx, executable, args...)
	if len(output) > maxAdapterOutput {
		return nil, errors.New("adapter result exceeded the safe output limit")
	}
	if err != nil {
		return nil, fmt.Errorf("adapter query failed: %s", safeCommandError(output))
	}
	var rows []map[string]any
	if json.Unmarshal(output, &rows) != nil {
		return nil, errors.New("adapter returned invalid structured output")
	}
	matches := make([]CapabilityMatch, 0, min(limit, len(rows)))
	for _, row := range rows {
		match := mapper(row)
		if match.Title == "" || !matchContains(match, query) {
			continue
		}
		matches = append(matches, match)
		if len(matches) == limit {
			break
		}
	}
	return matches, nil
}

func matchContains(match CapabilityMatch, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	return query == "" || strings.Contains(
		strings.ToLower(match.Title+" "+match.Summary+" "+match.Reference), query,
	)
}

func safeCommandError(output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) > 240 {
		text = text[:240]
	}
	if text == "" {
		return "provider command failed"
	}
	return text
}

func githubRepositoryMatch(row map[string]any) CapabilityMatch {
	return CapabilityMatch{
		Title: scalar(row, "fullName"), Summary: scalar(row, "description"),
		Reference: scalar(row, "url"),
		Metadata:  safeMetadata(row, "visibility", "updatedAt"),
	}
}

func githubPullRequestMatch(row map[string]any) CapabilityMatch {
	return CapabilityMatch{
		Title: scalar(row, "title"), Reference: scalar(row, "url"),
		Summary:  scalar(row, "state") + " pull request",
		Metadata: safeMetadata(row, "number", "state", "updatedAt"),
	}
}

func digitalOceanAppMatch(row map[string]any) CapabilityMatch {
	return CapabilityMatch{
		Title: nestedScalar(row, "spec", "name"), Reference: scalar(row, "id"),
		Summary:  nestedScalar(row, "active_deployment", "phase"),
		Metadata: map[string]any{"region": nestedScalar(row, "region", "slug")},
	}
}

func digitalOceanDropletMatch(row map[string]any) CapabilityMatch {
	return CapabilityMatch{
		Title: scalar(row, "name"), Reference: scalar(row, "id"),
		Summary:  scalar(row, "status"),
		Metadata: map[string]any{"region": nestedScalar(row, "region", "slug")},
	}
}

func scalar(row map[string]any, key string) string {
	switch value := row[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatInt(int64(value), 10)
	default:
		return ""
	}
}

func nestedScalar(row map[string]any, parent, key string) string {
	nested, _ := row[parent].(map[string]any)
	return scalar(nested, key)
}

func safeMetadata(row map[string]any, keys ...string) map[string]any {
	result := make(map[string]any)
	for _, key := range keys {
		if value := scalar(row, key); value != "" {
			result[key] = value
		}
	}
	return result
}

func postgresSchemaMatches(
	ctx context.Context, executable, profile, query string, limit int,
) ([]CapabilityMatch, error) {
	const statement = `SELECT table_schema,table_name,table_type FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema,table_name LIMIT 500`
	output, err := boundedCommandOutput(
		ctx, executable, "service="+profile, "--no-psqlrc", "--csv", "--tuples-only",
		"--set", "ON_ERROR_STOP=1", "--command", statement,
	)
	if len(output) > maxAdapterOutput {
		return nil, errors.New("database schema result exceeded the safe output limit")
	}
	if err != nil {
		return nil, fmt.Errorf("database schema query failed: %s", safeCommandError(output))
	}
	rows, err := csv.NewReader(strings.NewReader(string(output))).ReadAll()
	if err != nil {
		return nil, errors.New("database returned invalid schema output")
	}
	matches := make([]CapabilityMatch, 0, min(limit, len(rows)))
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		match := CapabilityMatch{
			Title: row[0] + "." + row[1], Summary: row[2],
			Reference: "postgres-schema://" + row[0] + "/" + row[1],
		}
		if matchContains(match, query) {
			matches = append(matches, match)
		}
		if len(matches) == limit {
			break
		}
	}
	return matches, nil
}

func boundedCommandOutput(ctx context.Context, executable string, args ...string) ([]byte, error) {
	var output bytes.Buffer
	writer := cappedCommandWriter{output: &output, limit: maxAdapterOutput + 1}
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdout, command.Stderr = &writer, &writer
	err := command.Run()
	return output.Bytes(), err
}

type cappedCommandWriter struct {
	output *bytes.Buffer
	limit  int
}

func (w *cappedCommandWriter) Write(value []byte) (int, error) {
	received := len(value)
	remaining := w.limit - w.output.Len()
	if remaining > 0 {
		_, _ = w.output.Write(value[:min(remaining, received)])
	}
	return received, nil
}
