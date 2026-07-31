package resourcegraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const uiVersion = "runspace.ui/v1"

type UIDocument struct {
	Version string         `json:"version"`
	Title   string         `json:"title"`
	Layout  map[string]any `json:"layout"`
}

type UIComponentDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

var uiComponents = []UIComponentDefinition{
	uiComponent("Markdown", "Safe GitHub-flavored explanatory content.",
		[]string{"content"}, map[string]any{"content": uiString("Markdown source, up to 20,000 characters.")}),
	uiComponent("CodeReference", "Clickable workspace file and symbol reference.",
		[]string{"resource", "path"}, map[string]any{
			"resource": uiString("Workspace Resource URI."), "path": uiString("Repository-relative file path."),
			"lineStart": uiInteger("Optional first line."), "lineEnd": uiInteger("Optional last line."),
		}),
	uiComponent("DiffViewer", "A workspace-backed source diff.",
		[]string{"resource"}, map[string]any{"resource": uiString("Snapshot or diff Resource URI.")}),
	uiComponent("TaskCard", "Task status, owner, and summary.",
		[]string{"title", "status"}, map[string]any{
			"title": uiString("Task title."), "status": uiString("Current task status."),
			"owner": uiString("Optional owner identity."), "summary": uiString("Optional task summary."),
		}),
	uiComponent("TestReport", "Structured test totals and failure groups.",
		[]string{"resource"}, map[string]any{
			"resource": uiString("Test-run Resource URI."), "title": uiString("Report title."),
			"passed": uiInteger("Passing test count."), "failed": uiInteger("Failing test count."),
		}),
	uiComponent("Timeline", "Ordered incident, release, or work events.",
		[]string{"items"}, map[string]any{"items": uiArray("Up to 50 timeline item objects.")}),
	uiComponent("DataTable", "Bounded tabular workspace information.",
		[]string{"columns", "rows"}, map[string]any{
			"columns": uiArray("Up to 12 {key,label} objects."), "rows": uiArray("Up to 100 row objects."),
		}),
	uiComponent("MetricGroup", "A compact group of labeled values.",
		[]string{"items"}, map[string]any{"items": uiArray("Up to 12 {label,value} objects.")}),
	uiComponent("ApprovalRequest", "A governed action requiring human review.",
		[]string{"title", "operation", "resource"}, map[string]any{
			"title": uiString("Approval title."), "operation": uiString("Registered workspace operation."),
			"resource": uiString("Target Resource URI."), "reason": uiString("Why approval is requested."),
		}),
	uiComponent("D3Artifact", "Declarative graph visualization with controlled actions.",
		[]string{"kind", "nodes", "edges"}, map[string]any{
			"kind":  uiString("Visualization kind such as dependency_graph."),
			"nodes": uiArray("Up to 100 {id,label,resource?} objects."),
			"edges": uiArray("Up to 200 {source,target} objects."),
		}),
}

func uiComponent(
	name, description string, required []string, properties map[string]any,
) UIComponentDefinition {
	return UIComponentDefinition{
		Name: name, Description: description,
		Schema: map[string]any{
			"type": "object", "required": required, "properties": properties,
			"additionalProperties": false,
		},
	}
}

func uiString(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func uiInteger(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func uiArray(description string) map[string]any {
	return map[string]any{"type": "array", "description": description}
}

func UIComponents() []UIComponentDefinition {
	result := make([]UIComponentDefinition, len(uiComponents))
	copy(result, uiComponents)
	return result
}

func UILayouts() []map[string]any {
	return []map[string]any{
		{"type": "Stack", "description": "Vertical component flow.", "max_children": 50},
		{"type": "Grid", "description": "Responsive one/two-column layout.", "max_children": 50},
	}
}

func ValidateUIDocument(document UIDocument) error {
	if document.Version != uiVersion || strings.TrimSpace(document.Title) == "" ||
		len(document.Title) > 160 || document.Layout == nil {
		return ErrInvalid
	}
	budget := 0
	return validateUINode(document.Layout, 0, &budget)
}

func validateUINode(node map[string]any, depth int, budget *int) error {
	if depth > 8 {
		return errors.New("UI artifact nesting exceeds eight levels")
	}
	*budget++
	if *budget > 100 {
		return errors.New("UI artifact exceeds 100 components")
	}
	nodeType, _ := node["type"].(string)
	if nodeType == "Stack" || nodeType == "Grid" {
		children, ok := node["children"].([]any)
		if !ok || len(children) == 0 || len(children) > 50 {
			return ErrInvalid
		}
		for _, child := range children {
			item, ok := child.(map[string]any)
			if !ok {
				return ErrInvalid
			}
			if err := validateUINode(item, depth+1, budget); err != nil {
				return err
			}
		}
		return nil
	}
	props, ok := node["props"].(map[string]any)
	if !ok || !knownUIComponent(nodeType) {
		return fmt.Errorf("%w: unknown UI component", ErrInvalid)
	}
	return validateUIProps(nodeType, props)
}

func knownUIComponent(name string) bool {
	for _, component := range uiComponents {
		if component.Name == name {
			return true
		}
	}
	return false
}

func validateUIProps(component string, props map[string]any) error {
	if encodedSize(props) > 64<<10 {
		return errors.New("UI component properties exceed 64 KiB")
	}
	switch component {
	case "Markdown":
		return requireText(props, "content", 20_000)
	case "CodeReference":
		return requireTexts(props, []string{"resource", "path"}, 500)
	case "DiffViewer", "TestReport":
		return requireText(props, "resource", 500)
	case "TaskCard":
		return requireTexts(props, []string{"title", "status"}, 500)
	case "Timeline":
		return boundedArray(props, "items", 50)
	case "MetricGroup":
		return boundedArray(props, "items", 12)
	case "DataTable":
		if err := boundedArray(props, "columns", 12); err != nil {
			return err
		}
		return boundedArray(props, "rows", 100)
	case "ApprovalRequest":
		return requireTexts(props, []string{"title", "operation", "resource"}, 500)
	case "D3Artifact":
		if err := requireText(props, "kind", 80); err != nil {
			return err
		}
		if props["kind"] != "dependency_graph" {
			return ErrInvalid
		}
		if err := boundedArray(props, "nodes", 100); err != nil {
			return err
		}
		return boundedArray(props, "edges", 200)
	default:
		return ErrInvalid
	}
}

func requireTexts(props map[string]any, keys []string, limit int) error {
	for _, key := range keys {
		if err := requireText(props, key, limit); err != nil {
			return err
		}
	}
	return nil
}

func requireText(props map[string]any, key string, limit int) error {
	value, ok := props[key].(string)
	if !ok || strings.TrimSpace(value) == "" || len(value) > limit {
		return ErrInvalid
	}
	return nil
}

func boundedArray(props map[string]any, key string, limit int) error {
	value, ok := props[key].([]any)
	if !ok || len(value) > limit {
		return ErrInvalid
	}
	return nil
}

func encodedSize(value any) int {
	encoded, _ := json.Marshal(value)
	return len(encoded)
}
