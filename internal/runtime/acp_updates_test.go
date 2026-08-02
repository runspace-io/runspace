package runtime

import (
	"encoding/json"
	"strings"
	"testing"
)

func describe(t *testing.T, raw string) (string, string) {
	t.Helper()
	return describeUpdate(json.RawMessage(raw))
}

func TestMessageChunkStillReadsAsProse(t *testing.T) {
	kind, text := describe(t, `{
		"sessionUpdate":"agent_message_chunk",
		"content":{"type":"text","text":"Reading main.go"}
	}`)
	if kind != "agent_message_chunk" || text != "Reading main.go" {
		t.Fatalf("kind=%q text=%q", kind, text)
	}
}

// A tool call is the agent's terminal activity. Its content is a list, so
// reading content.text produced "" and these were dropped entirely.
func TestToolCallSurfacesCommandAndOutput(t *testing.T) {
	kind, text := describe(t, `{
		"sessionUpdate":"tool_call",
		"title":"Run the tests",
		"status":"completed",
		"rawInput":{"command":"go","args":["test","./..."]},
		"content":[{"type":"content","content":{"type":"text","text":"ok  runspace 0.4s"}}]
	}`)
	if kind != NotificationToolCall {
		t.Fatalf("kind=%q", kind)
	}
	if !strings.Contains(text, "$ go test ./...") {
		t.Fatalf("command missing: %q", text)
	}
	if !strings.Contains(text, "ok  runspace 0.4s") {
		t.Fatalf("output missing: %q", text)
	}
}

func TestToolCallUpdateIsAlsoTerminalActivity(t *testing.T) {
	kind, text := describe(t, `{
		"sessionUpdate":"tool_call_update",
		"status":"failed",
		"rawInput":{"command":"rm -rf build/"}
	}`)
	if kind != NotificationToolCall || !strings.Contains(text, "$ rm -rf build/") {
		t.Fatalf("kind=%q text=%q", kind, text)
	}
}

// Without a command, the title is the most useful thing to show.
func TestToolCallFallsBackToItsTitle(t *testing.T) {
	_, text := describe(t, `{
		"sessionUpdate":"tool_call","title":"Search the repository","status":"in_progress"
	}`)
	if !strings.Contains(text, "$ Search the repository") {
		t.Fatalf("text=%q", text)
	}
}

// A status-only update still reports that something happened rather than
// vanishing, which is how a failure used to disappear.
func TestBareToolCallStatusIsStillReported(t *testing.T) {
	_, text := describe(t, `{"sessionUpdate":"tool_call","status":"failed"}`)
	if !strings.Contains(text, "failed") {
		t.Fatalf("text=%q", text)
	}
}

func TestMalformedUpdateIsIgnored(t *testing.T) {
	if kind, text := describe(t, `not json`); kind != "" || text != "" {
		t.Fatalf("kind=%q text=%q", kind, text)
	}
}
