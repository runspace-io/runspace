package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// permissionTimeout bounds how long an agent waits on a human. Expiry cancels
// the request, which is the pre-W2 behaviour — a question nobody answers must
// not pin an agent process forever.
const permissionTimeout = 10 * time.Minute

type pendingPermission struct {
	rpcID   int64
	options []PermissionOption
	timer   *time.Timer
}

// respondPermission handles an ACP session/request_permission call.
//
// It runs on the reader goroutine, so it must never block: parking the request
// and returning is what lets the peer keep streaming while a human decides. The
// JSON-RPC reply is written later by AnswerPermission or by the timeout.
func (c *stdioACP) respondPermission(id int64, raw []byte) {
	var request struct {
		Params struct {
			SessionID string `json:"sessionId"`
			ToolCall  struct {
				Title string `json:"title"`
				Kind  string `json:"kind"`
			} `json:"toolCall"`
			Options []struct {
				OptionID string `json:"optionId"`
				Name     string `json:"name"`
				Kind     string `json:"kind"`
			} `json:"options"`
		} `json:"params"`
	}
	_ = json.Unmarshal(raw, &request)
	options := make([]PermissionOption, 0, len(request.Params.Options))
	for _, option := range request.Params.Options {
		options = append(options, PermissionOption{
			ID: option.OptionID, Name: option.Name, Kind: option.Kind,
		})
	}
	if c.permissionMode == "yolo" {
		c.writeOutcome(id, autoSelected(options))
		return
	}
	// With nothing to choose between there is no question worth asking.
	if len(options) == 0 {
		c.writeOutcome(id, map[string]any{"outcome": "cancelled"})
		return
	}
	c.parkPermission(id, request.Params.SessionID, request.Params.ToolCall.Title, options)
}

func (c *stdioACP) parkPermission(
	id int64, sessionID, title string, options []PermissionOption,
) {
	c.mu.Lock()
	c.questionSeq++
	questionID := fmt.Sprintf("q_%d_%d", c.questionSeq, id)
	c.pendingPermissions[questionID] = pendingPermission{
		rpcID: id, options: options,
		timer: time.AfterFunc(permissionTimeout, func() { c.expirePermission(questionID) }),
	}
	c.mu.Unlock()
	payload, err := json.Marshal(PermissionRequest{
		QuestionID: questionID, Title: permissionTitle(title), Options: options,
	})
	if err != nil {
		_ = c.resolvePermission(questionID, "")
		return
	}
	select {
	case c.notifications <- ACPNotification{
		SessionID: sessionID, Kind: NotificationPermissionRequest,
		Text: permissionTitle(title), Payload: payload,
	}:
	default:
		// Nobody is listening, so nobody can answer. Fail closed now rather than
		// stall the agent until the timeout on a question no human will ever see.
		_ = c.resolvePermission(questionID, "")
	}
}

// AnswerPermission resolves a parked request. An empty optionID cancels it.
func (c *stdioACP) AnswerPermission(_ context.Context, questionID, optionID string) error {
	return c.resolvePermission(questionID, optionID)
}

// resolvePermission claims a parked request and replies to the peer exactly
// once. Validation happens before the entry is claimed: a rejected answer must
// leave the question open, or a typo would strand the agent until it times out.
func (c *stdioACP) resolvePermission(questionID, optionID string) error {
	c.mu.Lock()
	pending, found := c.pendingPermissions[questionID]
	if !found {
		c.mu.Unlock()
		return errors.New("permission request is no longer pending")
	}
	if optionID != "" && !offersOption(pending.options, optionID) {
		c.mu.Unlock()
		return fmt.Errorf("option %q was not offered", optionID)
	}
	delete(c.pendingPermissions, questionID)
	c.mu.Unlock()
	if pending.timer != nil {
		pending.timer.Stop()
	}
	outcome := map[string]any{"outcome": "cancelled"}
	if optionID != "" {
		outcome = map[string]any{"outcome": "selected", "optionId": optionID}
	}
	c.writeOutcome(pending.rpcID, outcome)
	return nil
}

func (c *stdioACP) expirePermission(questionID string) { _ = c.resolvePermission(questionID, "") }

// cancelPendingPermissions releases every parked request so a closing client
// leaves no timers behind.
func (c *stdioACP) cancelPendingPermissions() {
	c.mu.Lock()
	pending := c.pendingPermissions
	c.pendingPermissions = make(map[string]pendingPermission)
	c.mu.Unlock()
	for _, item := range pending {
		if item.timer != nil {
			item.timer.Stop()
		}
	}
}

func (c *stdioACP) writeOutcome(id int64, outcome map[string]any) {
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "result": map[string]any{"outcome": outcome},
	})
	c.mu.Lock()
	_, _ = c.stdin.Write(append(payload, '\n'))
	c.mu.Unlock()
}

func autoSelected(options []PermissionOption) map[string]any {
	for _, preferredKind := range []string{"allow_always", "allow_once"} {
		for _, option := range options {
			if option.Kind == preferredKind {
				return map[string]any{"outcome": "selected", "optionId": option.ID}
			}
		}
	}
	return map[string]any{"outcome": "cancelled"}
}

func offersOption(options []PermissionOption, optionID string) bool {
	for _, option := range options {
		if option.ID == optionID {
			return true
		}
	}
	return false
}

func permissionTitle(title string) string {
	if title == "" {
		return "The agent is asking for permission to continue."
	}
	return title
}
