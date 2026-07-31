package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

func TestMockRuntimeLifecycleAndStream(t *testing.T) {
	m := NewMockRuntime()
	ctx := context.Background()
	started, err := m.Spawn(ctx, contracts.SpawnRequest{RunID: "run-1", Prompt: "fix tests"})
	if err != nil || started.Status != contracts.RunQueued {
		t.Fatalf("spawn=%+v err=%v", started, err)
	}
	if _, err := m.Run(ctx, contracts.RunRequest{RunID: "run-1"}); err != nil {
		t.Fatal(err)
	}
	stream, err := m.Stream(ctx, contracts.StreamRequest{RunID: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for range stream {
		count++
	}
	if count != 3 {
		t.Fatalf("output count=%d", count)
	}
}

func TestMockRuntimeStopQueuedAndUnknown(t *testing.T) {
	m := NewMockRuntime()
	if err := m.Stop(context.Background(), contracts.StopRequest{RunID: "missing"}); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := m.Spawn(context.Background(), contracts.SpawnRequest{RunID: "queued"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background(), contracts.StopRequest{RunID: "queued"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Run(context.Background(), contracts.RunRequest{RunID: "queued"}); err == nil {
		t.Fatal("expected cancelled run to refuse start")
	}
}

func TestMockRuntimeCancellation(t *testing.T) {
	m := NewMockRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := m.Spawn(ctx, contracts.SpawnRequest{RunID: "cancel", Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Run(ctx, contracts.RunRequest{RunID: "cancel"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background(), contracts.StopRequest{RunID: "cancel"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-time.After(time.Second):
		t.Fatal("stream did not close")
	case <-func() <-chan struct{} {
		ch, _ := m.Stream(context.Background(), contracts.StreamRequest{RunID: "cancel"})
		done := make(chan struct{})
		go func() {
			for range ch {
			}
			close(done)
		}()
		return done
	}():
	}
}
