package resourceplugin

import (
	"context"
	"strings"

	"github.com/runspace/runspace/internal/resourcegraph"
)

type rowMapper func(map[string]any) resourcegraph.CapabilityMatch

func mapRows(rows []map[string]any, mapper rowMapper) []resourcegraph.CapabilityMatch {
	result := make([]resourcegraph.CapabilityMatch, 0, len(rows))
	for _, row := range rows {
		match := mapper(row)
		if match.Title != "" {
			result = append(result, match)
		}
	}
	return result
}

func filterMatches(
	matches []resourcegraph.CapabilityMatch, query string, limit int,
) []resourcegraph.CapabilityMatch {
	needle := strings.ToLower(strings.TrimSpace(query))
	result := make([]resourcegraph.CapabilityMatch, 0, min(limit, len(matches)))
	for _, match := range matches {
		haystack := strings.ToLower(match.Title + " " + match.Summary + " " + match.Reference)
		if needle == "" || strings.Contains(haystack, needle) {
			result = append(result, match)
		}
		if len(result) == limit {
			break
		}
	}
	return result
}

func githubRepository(row map[string]any) resourcegraph.CapabilityMatch {
	return resourcegraph.CapabilityMatch{
		Title: text(row, "full_name"), Summary: text(row, "description"),
		Reference: text(row, "html_url"),
		Metadata:  pick(row, "visibility", "updated_at"),
	}
}

func githubPullRequest(row map[string]any) resourcegraph.CapabilityMatch {
	return resourcegraph.CapabilityMatch{
		Title: text(row, "title"), Summary: text(row, "state") + " pull request",
		Reference: text(row, "html_url"), Metadata: pick(row, "number", "updated_at"),
	}
}

func digitalOceanApp(row map[string]any) resourcegraph.CapabilityMatch {
	return resourcegraph.CapabilityMatch{
		Title: nestedText(row, "spec", "name"), Reference: text(row, "id"),
		Summary:  nestedText(row, "active_deployment", "phase"),
		Metadata: map[string]any{"region": nestedText(row, "region", "slug")},
	}
}

func digitalOceanDroplet(row map[string]any) resourcegraph.CapabilityMatch {
	return resourcegraph.CapabilityMatch{
		Title: text(row, "name"), Reference: text(row, "id"), Summary: text(row, "status"),
		Metadata: map[string]any{"region": nestedText(row, "region", "slug")},
	}
}

func text(row map[string]any, key string) string {
	value, _ := row[key].(string)
	return strings.TrimSpace(value)
}

func nestedText(row map[string]any, parent, key string) string {
	nested, _ := row[parent].(map[string]any)
	return text(nested, key)
}

func pick(row map[string]any, keys ...string) map[string]any {
	result := make(map[string]any)
	for _, key := range keys {
		switch value := row[key].(type) {
		case string:
			result[key] = value
		case float64:
			result[key] = int64(value)
		}
	}
	return result
}

func (s *Service) postgresSchema(
	ctx context.Context, credential, query string, limit int,
) ([]resourcegraph.CapabilityMatch, error) {
	database, err := s.providers.openPostgres(credential)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	const statement = `SELECT table_schema,table_name,table_type FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema,table_name LIMIT 500`
	rows, err := database.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matches := make([]resourcegraph.CapabilityMatch, 0)
	for rows.Next() {
		var schema, table, kind string
		if err := rows.Scan(&schema, &table, &kind); err != nil {
			return nil, err
		}
		matches = append(matches, resourcegraph.CapabilityMatch{
			Title: schema + "." + table, Summary: kind,
			Reference: "postgres-schema://" + schema + "/" + table,
		})
	}
	return filterMatches(matches, query, limit), rows.Err()
}

func defaultText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
