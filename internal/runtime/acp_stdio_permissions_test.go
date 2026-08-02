package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

type bufferWriteCloser struct{ bytes.Buffer }

func (*bufferWriteCloser) Close() error { return nil }

type permissionOutcome struct {
	Result struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	} `json:"result"`
}

func decodeOutcome(t *testing.T, raw []byte) permissionOutcome {
	t.Helper()
	var response permissionOutcome
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return response
}

func permissionClient(mode string) (*stdioACP, *bufferWriteCloser) {
	output := &bufferWriteCloser{}
	return &stdioACP{
		stdin: output, permissionMode: mode,
		notifications:      make(chan ACPNotification, 4),
		pendingPermissions: make(map[string]pendingPermission),
	}, output
}

const askRequest = `{
	"params":{
		"sessionId":"session-1",
		"toolCall":{"title":"Run rm -rf build/","kind":"execute"},
		"options":[
			{"optionId":"reject","name":"Reject","kind":"reject_once"},
			{"optionId":"once","name":"Allow once","kind":"allow_once"}
		]
	}
}`

func TestYoloSelectsAgentPermissionOption(t *testing.T) {
	client, output := permissionClient("yolo")
	client.respondPermission(7, []byte(askRequest))
	response := decodeOutcome(t, output.Bytes())
	if response.Result.Outcome.Outcome != "selected" ||
		response.Result.Outcome.OptionID != "once" {
		t.Fatalf("response=%s", output.String())
	}
}

// The reader goroutine dispatches permission calls, so parking one must not
// write a reply or block; the agent keeps streaming while a human decides.
func TestDefaultModeParksQuestionInsteadOfCancelling(t *testing.T) {
	client, output := permissionClient("default")
	client.respondPermission(7, []byte(askRequest))
	if output.Len() != 0 {
		t.Fatalf("parked question answered itself: %s", output.String())
	}
	notification := <-client.Notifications()
	if notification.Kind != NotificationPermissionRequest ||
		notification.SessionID != "session-1" {
		t.Fatalf("unexpected notification: %#v", notification)
	}
	var request PermissionRequest
	if err := json.Unmarshal(notification.Payload, &request); err != nil {
		t.Fatal(err)
	}
	if request.QuestionID == "" || request.Title != "Run rm -rf build/" ||
		len(request.Options) != 2 || request.Options[1].Name != "Allow once" {
		t.Fatalf("question lost detail: %#v", request)
	}
	if err := client.AnswerPermission(context.Background(), request.QuestionID, "once"); err != nil {
		t.Fatal(err)
	}
	response := decodeOutcome(t, output.Bytes())
	if response.Result.Outcome.Outcome != "selected" ||
		response.Result.Outcome.OptionID != "once" {
		t.Fatalf("answer not forwarded: %s", output.String())
	}
}

func TestAnsweringWithNoOptionCancels(t *testing.T) {
	client, output := permissionClient("default")
	client.respondPermission(7, []byte(askRequest))
	request := parkedRequest(t, client)
	if err := client.AnswerPermission(context.Background(), request.QuestionID, ""); err != nil {
		t.Fatal(err)
	}
	if decodeOutcome(t, output.Bytes()).Result.Outcome.Outcome != "cancelled" {
		t.Fatalf("response=%s", output.String())
	}
}

// An answer naming an option the agent never offered must not reach the peer.
func TestAnswerRejectsUnofferedOption(t *testing.T) {
	client, output := permissionClient("default")
	client.respondPermission(7, []byte(askRequest))
	request := parkedRequest(t, client)
	if err := client.AnswerPermission(
		context.Background(), request.QuestionID, "allow_always",
	); err == nil {
		t.Fatal("accepted an option the agent never offered")
	}
	if output.Len() != 0 {
		t.Fatalf("forged option reached the agent: %s", output.String())
	}
	// The question stays answerable after a rejected attempt.
	if err := client.AnswerPermission(
		context.Background(), request.QuestionID, "reject",
	); err != nil {
		t.Fatal(err)
	}
}

func TestAnsweringTwiceFails(t *testing.T) {
	client, _ := permissionClient("default")
	client.respondPermission(7, []byte(askRequest))
	request := parkedRequest(t, client)
	if err := client.AnswerPermission(context.Background(), request.QuestionID, "once"); err != nil {
		t.Fatal(err)
	}
	if err := client.AnswerPermission(
		context.Background(), request.QuestionID, "reject",
	); err == nil {
		t.Fatal("a resolved question accepted a second answer")
	}
}

// With nobody consuming notifications the question can never be seen, so the
// agent must be released immediately rather than hang until the timeout.
func TestUnobservedQuestionFailsClosed(t *testing.T) {
	output := &bufferWriteCloser{}
	client := &stdioACP{
		stdin: output, permissionMode: "default",
		notifications:      make(chan ACPNotification), // unbuffered, no reader
		pendingPermissions: make(map[string]pendingPermission),
	}
	client.respondPermission(7, []byte(askRequest))
	if decodeOutcome(t, output.Bytes()).Result.Outcome.Outcome != "cancelled" {
		t.Fatalf("response=%s", output.String())
	}
}

func TestRequestWithoutOptionsIsCancelled(t *testing.T) {
	client, output := permissionClient("default")
	client.respondPermission(7, []byte(`{"params":{"sessionId":"s","options":[]}}`))
	if decodeOutcome(t, output.Bytes()).Result.Outcome.Outcome != "cancelled" {
		t.Fatalf("response=%s", output.String())
	}
}

func parkedRequest(t *testing.T, client *stdioACP) PermissionRequest {
	t.Helper()
	notification := <-client.Notifications()
	var request PermissionRequest
	if err := json.Unmarshal(notification.Payload, &request); err != nil {
		t.Fatal(err)
	}
	return request
}
