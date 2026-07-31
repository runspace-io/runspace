package resourcegraph

import (
	"encoding/json"
	"testing"
)

func TestRunspaceUIDocumentValidation(t *testing.T) {
	var document UIDocument
	if err := json.Unmarshal([]byte(`{
		"version":"runspace.ui/v1",
		"title":"Release status",
		"layout":{"type":"Stack","children":[
			{"type":"MetricGroup","props":{"items":[{"label":"Passing","value":"92%"}]}},
			{"type":"D3Artifact","props":{"kind":"dependency_graph","nodes":[],"edges":[]}}
		]}
	}`), &document); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUIDocument(document); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}
	document.Layout = map[string]any{
		"type": "RawJavaScript", "props": map[string]any{"code": "fetch('/secrets')"},
	}
	if ValidateUIDocument(document) == nil {
		t.Fatal("arbitrary executable component was accepted")
	}
}
