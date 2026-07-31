package publish

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

var branchPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)

type LocalGit interface {
	Status(context.Context, string) (string, error)
	CreateBranch(context.Context, contracts.BranchRequest) (contracts.BranchResult, error)
	Commit(context.Context, string, string) (string, error)
	Push(context.Context, string, string, string) error
}

type Remote interface {
	OpenPR(context.Context, contracts.PullRequest) (contracts.PullRequestResult, error)
}

type Request struct {
	ID, RepositoryPath, Repository, Branch, Base, CommitMessage string
	Title, Body                                                 string
}

type Result struct {
	ID          string                      `json:"id"`
	Branch      contracts.BranchResult      `json:"branch"`
	CommitSHA   string                      `json:"commit_sha"`
	PullRequest contracts.PullRequestResult `json:"pull_request"`
	CreatedAt   time.Time                   `json:"created_at"`
}

type Service struct {
	mu      sync.Mutex
	local   LocalGit
	remote  Remote
	results map[string]Result
	now     func() time.Time
}

func New(local LocalGit, remote Remote) *Service {
	return &Service{local: local, remote: remote, results: make(map[string]Result), now: time.Now}
}

func (s *Service) Publish(ctx context.Context, request Request) (Result, error) {
	if err := validate(request); err != nil {
		return Result{}, err
	}
	s.mu.Lock()
	if result, ok := s.results[request.ID]; ok {
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()
	if s.local == nil || s.remote == nil {
		return Result{}, errors.New("publish dependencies are required")
	}
	branch, commit, err := prepareChanges(ctx, s.local, request)
	if err != nil {
		return Result{}, err
	}
	pr, err := s.remote.OpenPR(ctx, contracts.PullRequest{Repository: request.Repository, Title: request.Title, Body: request.Body, Head: branch.Name, Base: request.Base})
	if err != nil {
		return Result{}, fmt.Errorf("open pull request: %w", err)
	}
	result := Result{ID: request.ID, Branch: branch, CommitSHA: strings.TrimSpace(commit), PullRequest: pr, CreatedAt: s.now().UTC()}
	s.mu.Lock()
	s.results[request.ID] = result
	s.mu.Unlock()
	return result, nil
}

func prepareChanges(ctx context.Context, local LocalGit, request Request) (contracts.BranchResult, string, error) {
	status, err := local.Status(ctx, request.RepositoryPath)
	if err != nil {
		return contracts.BranchResult{}, "", fmt.Errorf("inspect changes: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return contracts.BranchResult{}, "", errors.New("no changes to publish")
	}
	branch, err := local.CreateBranch(ctx, contracts.BranchRequest{Repository: request.RepositoryPath, Name: request.Branch, From: request.Base})
	if err != nil {
		return contracts.BranchResult{}, "", fmt.Errorf("create branch: %w", err)
	}
	commit, err := local.Commit(ctx, request.RepositoryPath, request.CommitMessage)
	if err != nil {
		return contracts.BranchResult{}, "", fmt.Errorf("commit changes: %w", err)
	}
	if err := local.Push(ctx, request.RepositoryPath, "origin", branch.Name); err != nil {
		return contracts.BranchResult{}, "", fmt.Errorf("push branch: %w", err)
	}
	return branch, strings.TrimSpace(commit), nil
}

func validate(request Request) error {
	for name, value := range map[string]string{"id": request.ID, "repository path": request.RepositoryPath, "repository": request.Repository, "base": request.Base, "commit message": request.CommitMessage, "title": request.Title} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if !branchPattern.MatchString(request.Branch) || strings.Contains(request.Branch, "..") {
		return errors.New("invalid branch name")
	}
	return nil
}
