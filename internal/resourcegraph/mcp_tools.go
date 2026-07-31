package resourcegraph

func graphTools() []map[string]any {
	return []map[string]any{
		graphTool("search_resources", "Search normalized workspace resources with evidence-backed metadata.",
			objectSchema(map[string]any{
				"query":     stringSchema("Text to search in titles and summaries."),
				"thread_id": stringSchema("Optional discussion thread override."),
				"kind": stringEnumSchema("Optional node kind.", []string{
					"resource", "task", "artifact", "action", "discussion", "identity", "policy", "event",
				}),
				"limit": numberSchema("Maximum results, up to 200."),
			}, []string{"query"})),
		graphTool("query_resource", "Query a shared Resource through its owner-hosted, allowlisted capability.",
			objectSchema(map[string]any{
				"node_id":    stringSchema("Namespaced Resource graph node ID."),
				"capability": stringSchema("One capability advertised in the Resource metadata."),
				"query":      stringSchema("Provider search text; never a shell command or raw SQL."),
				"limit":      numberSchema("Maximum structured matches."),
			}, []string{"node_id", "capability"})),
		graphTool("read_context", "Read one node and its incoming and outgoing relationships.",
			objectSchema(map[string]any{"node_id": stringSchema("Namespaced graph node ID.")}, []string{"node_id"})),
		graphTool("read_discussion", "Read recent messages from the current Runspace discussion.",
			objectSchema(map[string]any{
				"thread_id": stringSchema("Optional thread override; defaults to the connected discussion."),
				"limit":     numberSchema("Maximum recent messages, up to 200. Defaults to 50."),
			}, nil)),
		graphTool("send_message", "Send a message to the connected discussion as this ACP agent.",
			objectSchema(map[string]any{
				"body": stringSchema("Share-safe message body for channel members."),
			}, []string{"body"})),
		graphTool("list_tasks", "List shared workspace tasks, optionally scoped to a discussion thread.",
			objectSchema(map[string]any{"thread_id": stringSchema("Optional Runspace thread ID.")}, nil)),
		graphTool("create_task", "Create a shared coordination task without launching an AI chat.",
			objectSchema(map[string]any{
				"title": stringSchema("Task title."), "summary": stringSchema("Task purpose and outcome."),
				"thread_id":     stringSchema("Optional discussion thread ID."),
				"discussion_id": stringSchema("Optional namespaced discussion node ID."),
			}, []string{"title"})),
		graphTool("publish_artifact", "Publish a safe artifact reference into the workspace graph.",
			objectSchema(map[string]any{
				"title": stringSchema("Artifact title."), "type": stringSchema("Artifact subtype."),
				"summary":      stringSchema("Share-safe description."),
				"external_ref": stringSchema("Optional external URI or provider reference."),
				"thread_id":    stringSchema("Optional discussion thread ID."),
				"task_id":      stringSchema("Optional namespaced task node ID."),
			}, []string{"title"})),
		graphTool("request_access", "Create a pending governed capability request for a human to review.",
			objectSchema(map[string]any{
				"resource_id": stringSchema("Target resource ID."),
				"capability":  stringSchema("Capability being requested."),
				"reason":      stringSchema("Why access is needed."),
			}, []string{"resource_id", "capability", "reason"})),
		graphTool("ui.list_components", "List approved reusable UI components and the artifact version.",
			objectSchema(map[string]any{}, nil)),
		graphTool("ui.get_component_schema", "Read one approved component schema before using it.",
			objectSchema(map[string]any{
				"component": stringSchema("Exact component name returned by ui.list_components."),
			}, []string{"component"})),
		graphTool("ui.create_artifact", "Validate, persist, and share a runspace.ui/v1 artifact.",
			objectSchema(map[string]any{
				"document":  uiDocumentSchema(),
				"thread_id": stringSchema("Optional discussion thread override."),
			}, []string{"document"})),
		graphTool("ui.update_artifact", "Update an owned UI artifact with a validated document.",
			objectSchema(map[string]any{
				"node_id":  stringSchema("Existing UI artifact graph node ID."),
				"document": uiDocumentSchema(),
			}, []string{"node_id", "document"})),
		graphTool("ui.request_action", "Create a governed request from an interactive artifact.",
			objectSchema(map[string]any{
				"operation": stringSchema("Registered workspace operation."),
				"resource":  stringSchema("Target workspace Resource URI."),
				"reason":    stringSchema("Reason for requesting the action."),
			}, []string{"operation", "resource"})),
	}
}

func uiDocumentSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "A complete validated runspace.ui/v1 declarative document.",
		"additionalProperties": true,
	}
}

func graphTool(name, description string, schema map[string]any) map[string]any {
	return map[string]any{"name": name, "description": description, "inputSchema": schema}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type": "object", "properties": properties, "additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func stringEnumSchema(description string, values []string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func numberSchema(description string) map[string]any {
	return map[string]any{
		"type": "integer", "description": description, "minimum": 1, "maximum": 200,
	}
}
