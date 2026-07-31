package publish

import (
	"context"
	"testing"

	"github.com/runspace/runspace/internal/contracts"
)

type fakeLocal struct{ calls int }

func (f *fakeLocal) Status(context.Context, string) (string, error) { return " M file.go", nil }
func (f *fakeLocal) CreateBranch(context.Context, contracts.BranchRequest) (contracts.BranchResult, error) {
	f.calls++
	return contracts.BranchResult{Name: "forge/run-1", SHA: "base"}, nil
}
func (f *fakeLocal) Commit(context.Context, string, string) (string, error) {
	f.calls++
	return "commit-sha", nil
}
func (f *fakeLocal) Push(context.Context, string, string, string) error { f.calls++; return nil }

type fakeRemote struct{ calls int }

func (f *fakeRemote) OpenPR(context.Context, contracts.PullRequest) (contracts.PullRequestResult, error) {
	f.calls++
	return contracts.PullRequestResult{Number: 1, URL: "https://example/pr/1"}, nil
}

func TestPublishIsIdempotent(t *testing.T) {
	local := &fakeLocal{}
	remote := &fakeRemote{}
	service := New(local, remote)
	request := Request{ID: "publish-1", RepositoryPath: t.TempDir(), Repository: "org/repo", Branch: "forge/run-1", Base: "main", CommitMessage: "feat: change", Title: "Change", Body: "Details"}
	first, err := service.Publish(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Publish(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.PullRequest.URL != second.PullRequest.URL || local.calls != 3 || remote.calls != 1 {
		t.Fatalf("first=%+v second=%+v local=%d remote=%d", first, second, local.calls, remote.calls)
	}
}

func TestPublishRejectsEmptyChangesAndUnsafeBranch(t *testing.T) {
	if err := validate(Request{ID: "x", RepositoryPath: "p", Repository: "r", Branch: "../x", Base: "main", CommitMessage: "m", Title: "t"}); err == nil {
		t.Fatal("expected invalid branch")
	}
	service := New(&emptyLocal{}, &fakeRemote{})
	_, err := service.Publish(context.Background(), Request{ID: "x", RepositoryPath: "p", Repository: "r", Branch: "forge/x", Base: "main", CommitMessage: "m", Title: "t"})
	if err == nil || err.Error() != "no changes to publish" {
		t.Fatalf("err=%v", err)
	}
}

type emptyLocal struct{}

func (*emptyLocal) Status(context.Context, string) (string, error) { return "", nil }
func (*emptyLocal) CreateBranch(context.Context, contracts.BranchRequest) (contracts.BranchResult, error) {
	return contracts.BranchResult{}, nil
}
func (*emptyLocal) Commit(context.Context, string, string) (string, error) { return "", nil }
func (*emptyLocal) Push(context.Context, string, string, string) error     { return nil }
