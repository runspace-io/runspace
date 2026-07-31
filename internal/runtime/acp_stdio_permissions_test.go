package runtime

import (
	"bytes"
	"encoding/json"
	"testing"
)

type bufferWriteCloser struct{ bytes.Buffer }

func (*bufferWriteCloser) Close() error { return nil }

func TestYoloSelectsAgentPermissionOption(t *testing.T) {
	output := &bufferWriteCloser{}
	client := &stdioACP{stdin: output, permissionMode: "yolo"}
	client.respondPermission(7, []byte(`{
		"params":{"options":[
			{"optionId":"reject","kind":"reject_once"},
			{"optionId":"once","kind":"allow_once"}
		]}
	}`))
	var response struct {
		Result struct {
			Outcome struct {
				Outcome  string `json:"outcome"`
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result.Outcome.Outcome != "selected" ||
		response.Result.Outcome.OptionID != "once" {
		t.Fatalf("response=%s", output.String())
	}
}
