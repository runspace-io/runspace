package resourceplugin

var manifests = []Manifest{
	{
		ID: "github", Name: "GitHub",
		Description:  "Connect GitHub directly through its API without a local CLI.",
		ResourceType: "github_account", Placements: []string{"runspace", "connector"},
		AuthMethods: []AuthMethod{
			{ID: "token", Label: "Personal access token", SecretLabel: "GitHub token", Placeholder: "github_pat_…"},
		},
		Capabilities: []Capability{
			query("github.repositories.query", "Search repositories", "Search repository metadata visible to this token."),
			query("github.pull_requests.query", "Search pull requests", "Search pull requests visible to this token."),
		},
	},
	{
		ID: "digitalocean", Name: "DigitalOcean",
		Description:  "Manage DigitalOcean resources through a native API connection.",
		ResourceType: "digitalocean_account", Placements: []string{"runspace", "connector"},
		AuthMethods: []AuthMethod{
			{ID: "token", Label: "API token", SecretLabel: "DigitalOcean token", Placeholder: "dop_v1_…"},
		},
		Capabilities: []Capability{
			query("digitalocean.apps.query", "Query apps", "Search application deployment metadata."),
			query("digitalocean.droplets.query", "Query droplets", "Search droplet status and region metadata."),
		},
	},
	{
		ID: "postgresql", Name: "PostgreSQL",
		Description:  "Explore a reachable PostgreSQL database using a fixed schema query.",
		ResourceType: "postgres_database", Placements: []string{"runspace", "connector"},
		AuthMethods: []AuthMethod{
			{ID: "connection_url", Label: "Connection URL", SecretLabel: "PostgreSQL URL", Placeholder: "postgresql://…"},
		},
		Capabilities: []Capability{
			query("postgres.schema.query", "Query schema", "Search tables and views without reading row data."),
		},
	},
}

func query(id, label, description string) Capability {
	return Capability{ID: id, Label: label, Description: description, Mode: "query", Risk: "read"}
}

func Manifests() []Manifest {
	result := make([]Manifest, len(manifests))
	copy(result, manifests)
	return result
}

func manifest(id string) (Manifest, bool) {
	for _, item := range manifests {
		if item.ID == id {
			return item, true
		}
	}
	return Manifest{}, false
}

func hasCapability(item Manifest, id string) bool {
	for _, capability := range item.Capabilities {
		if capability.ID == id {
			return true
		}
	}
	return false
}
