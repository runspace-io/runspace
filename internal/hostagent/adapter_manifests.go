package hostagent

var builtinAdapterManifests = []AdapterManifest{
	{
		ID: "github-cli", Name: "GitHub CLI",
		Description: "Query repositories and pull requests using the owner's existing gh login.",
		Executable:  "gh", ResourceType: "github_account",
		Capabilities: []CapabilityDescriptor{
			queryCapability("github.repositories.query", "Search repositories", "Search safe repository metadata."),
			queryCapability("github.pull_requests.query", "Search pull requests", "Search pull requests visible to this profile."),
		},
	},
	{
		ID: "postgresql", Name: "PostgreSQL",
		Description: "Explore database schemas through an existing psql service profile.",
		Executable:  "psql", ResourceType: "postgres_database",
		Capabilities: []CapabilityDescriptor{
			queryCapability("postgres.schema.query", "Query schema", "Search tables and views without reading row data."),
		},
	},
	{
		ID: "digitalocean-cli", Name: "DigitalOcean CLI",
		Description: "Query apps and droplets using the owner's existing doctl context.",
		Executable:  "doctl", ResourceType: "digitalocean_account",
		Capabilities: []CapabilityDescriptor{
			queryCapability("digitalocean.apps.query", "Query apps", "Search application deployment metadata."),
			queryCapability("digitalocean.droplets.query", "Query droplets", "Search droplet status and region metadata."),
		},
	},
}

func queryCapability(id, label, description string) CapabilityDescriptor {
	return CapabilityDescriptor{
		ID: id, Label: label, Description: description, Mode: "query", Risk: "read",
	}
}

func adapterManifest(id string) (AdapterManifest, bool) {
	for _, manifest := range builtinAdapterManifests {
		if manifest.ID == id {
			manifest.Capabilities = append([]CapabilityDescriptor(nil), manifest.Capabilities...)
			return manifest, true
		}
	}
	return AdapterManifest{}, false
}

func capabilityAllowed(manifest AdapterManifest, capability string) bool {
	for _, item := range manifest.Capabilities {
		if item.ID == capability {
			return true
		}
	}
	return false
}
