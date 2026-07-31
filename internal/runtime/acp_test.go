package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

type fakeACP struct {
	notices   chan ACPNotification
	session   string
	cwd       string
	cancelled bool
	prompts   []string
	calls     chan struct{}
}

func (f *fakeACP) Initialize(context.Context) error { return nil }
func (f *fakeACP) NewSession(_ context.Context, cwd string) (string, error) {
	f.session = "s1"
	f.cwd = cwd
	return f.session, nil
}
func (f *fakeACP) ResumeSession(context.Context, string, string) error { return nil }
func (f *fakeACP) SetSessionModel(context.Context, string, string) error {
	return nil
}
func (f *fakeACP) Prompt(context.Context, string, string) error {
	// The test peer records both the initial and follow-up prompts.
	f.prompts = append(f.prompts, "prompt")
	if f.calls != nil {
		f.calls <- struct{}{}
	}
	f.notices <- ACPNotification{SessionID: "s1", Kind: "text", Text: "hello"}
	return nil
}

func TestACPSendRoutesFollowUpToActiveSession(t *testing.T) {
	fake := &fakeACP{notices: make(chan ACPNotification, 4), calls: make(chan struct{}, 4)}
	a := NewACP(func(context.Context) (ACPClient, error) { return fake, nil })
	_, _ = a.Spawn(context.Background(), contracts.SpawnRequest{RunID: "r1"})
	if _, err := a.Run(context.Background(), contracts.RunRequest{RunID: "r1"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Send(context.Background(), contracts.InputRequest{RunID: "r1", Text: "continue"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-fake.calls:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for ACP prompts")
		}
	}
}

func TestACPSendRejectsInactiveOrInvalidInput(t *testing.T) {
	a := NewACP(nil)
	if err := a.Send(context.Background(), contracts.InputRequest{RunID: "missing", Text: "x"}); err == nil {
		t.Fatal("expected missing run error")
	}
	_, _ = a.Spawn(context.Background(), contracts.SpawnRequest{RunID: "queued"})
	if err := a.Send(context.Background(), contracts.InputRequest{RunID: "queued", Text: "x"}); err == nil {
		t.Fatal("expected inactive session error")
	}
	if err := a.Send(context.Background(), contracts.InputRequest{RunID: "queued"}); err == nil {
		t.Fatal("expected empty input error")
	}
}
func (f *fakeACP) Cancel(context.Context, string) error  { f.cancelled = true; return nil }
func (f *fakeACP) Notifications() <-chan ACPNotification { return f.notices }
func (f *fakeACP) Close() error                          { return nil }

func TestACPStreamsSessionUpdates(t *testing.T) {
	fake := &fakeACP{notices: make(chan ACPNotification, 1)}
	a := NewACP(func(context.Context) (ACPClient, error) { return fake, nil })
	if _, err := a.Spawn(context.Background(), contracts.SpawnRequest{
		RunID: "r1", Prompt: "hi", WorkingDirectory: "/workspace/repository",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), contracts.RunRequest{RunID: "r1"}); err != nil {
		t.Fatal(err)
	}
	stream, err := a.Stream(context.Background(), contracts.StreamRequest{RunID: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	output := <-stream
	if output.Text != "hello" || output.RunID != "r1" {
		t.Fatalf("unexpected output: %+v", output)
	}
	if fake.cwd != "/workspace/repository" {
		t.Fatalf("ACP session cwd=%q", fake.cwd)
	}
}

func TestACPStopCancelsSession(t *testing.T) {
	fake := &fakeACP{notices: make(chan ACPNotification)}
	a := NewACP(func(context.Context) (ACPClient, error) { return fake, nil })
	_, _ = a.Spawn(context.Background(), contracts.SpawnRequest{RunID: "r1"})
	if _, err := a.Run(context.Background(), contracts.RunRequest{RunID: "r1"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Stop(context.Background(), contracts.StopRequest{RunID: "r1"}); err != nil {
		t.Fatal(err)
	}
	if !fake.cancelled {
		t.Fatal("expected ACP cancellation")
	}
}
