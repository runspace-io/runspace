package runtime

import (
	"encoding/json"
	"strings"
)

// NotificationToolCall marks a notification describing a command the agent ran
// rather than something it said.
const NotificationToolCall = "tool_call"

// sessionUpdate covers the shapes ACP uses for one session/update. Message
// chunks carry a single content object; tool calls carry a list, plus the
// command and its status.
type sessionUpdate struct {
	Kind     string          `json:"sessionUpdate"`
	Content  json.RawMessage `json:"content"`
	Title    string          `json:"title"`
	Status   string          `json:"status"`
	RawInput struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"rawInput"`
}

// describeUpdate renders an update as the line a human should see.
//
// Only message chunks used to survive: a tool call puts its content in a list,
// so reading content.text produced "" and the agent's actual commands were
// dropped. Those commands are the agent's terminal activity, which is exactly
// what a reviewer needs to see.
func describeUpdate(raw json.RawMessage) (kind string, text string) {
	var update sessionUpdate
	if json.Unmarshal(raw, &update) != nil {
		return "", ""
	}
	if !strings.HasPrefix(update.Kind, NotificationToolCall) {
		return update.Kind, contentText(update.Content)
	}
	return NotificationToolCall, toolCallText(update)
}

func toolCallText(update sessionUpdate) string {
	lines := make([]string, 0, 3)
	if command := commandLine(update); command != "" {
		lines = append(lines, "$ "+command)
	} else if title := strings.TrimSpace(update.Title); title != "" {
		lines = append(lines, "$ "+title)
	}
	if output := strings.TrimSpace(contentText(update.Content)); output != "" {
		lines = append(lines, output)
	}
	// A bare status is still worth showing: it is how a failed command reads.
	if len(lines) == 0 && strings.TrimSpace(update.Status) != "" {
		return "$ (tool call " + update.Status + ")"
	}
	return strings.Join(lines, "\n")
}

func commandLine(update sessionUpdate) string {
	command := strings.TrimSpace(update.RawInput.Command)
	if command == "" {
		return ""
	}
	if len(update.RawInput.Args) == 0 {
		return command
	}
	return command + " " + strings.Join(update.RawInput.Args, " ")
}

// contentText flattens either a single content object or a list of them, which
// is how ACP encodes a message chunk versus a tool call's output.
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var single struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &single) == nil && single.Text != "" {
		return single.Text
	}
	var many []struct {
		Text    string `json:"text"`
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &many) != nil {
		return ""
	}
	parts := make([]string, 0, len(many))
	for _, item := range many {
		if text := strings.TrimSpace(fallbackText(item.Text, item.Content.Text)); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func fallbackText(primary, secondary string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return secondary
}
