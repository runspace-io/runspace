package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/runspace/runspace/internal/contracts"
)

var ErrUnsupported = errors.New("git operation requires a hosting provider")

// CommandRunner executes git without a shell. Keeping this boundary small makes
// command construction testable and prevents argument injection.
type CommandRunner interface {
	Run(context.Context, string, ...string) (string, error)
}

type nativeRunner struct{}

func (nativeRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	commandArgs := args
	if dir != "" {
		commandArgs = append([]string{"-c", "safe.directory=" + dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", commandArgs...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w: %s", strings.Join(commandArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

type Provider struct{ runner CommandRunner }

func NewProvider() Provider { return Provider{runner: nativeRunner{}} }

func NewProviderWithRunner(r CommandRunner) Provider { return Provider{runner: r} }

var _ contracts.GitProvider = Provider{}

func (p Provider) Clone(ctx context.Context, req contracts.CloneRequest) (contracts.CloneResult, error) {
	if req.RepositoryURL == "" || req.Destination == "" {
		return contracts.CloneResult{}, errors.New("repository URL and destination are required")
	}
	if _, err := os.Stat(req.Destination); err == nil {
		return contracts.CloneResult{}, errors.New("destination already exists")
	} else if !os.IsNotExist(err) {
		return contracts.CloneResult{}, err
	}
	args := []string{"clone"}
	if req.Ref != "" {
		args = append(args, "--branch", req.Ref)
	}
	args = append(args, req.RepositoryURL, req.Destination)
	if _, err := p.runner.Run(ctx, "", args...); err != nil {
		return contracts.CloneResult{}, err
	}
	ref := req.Ref
	if ref == "" {
		ref = "HEAD"
	}
	return contracts.CloneResult{Path: req.Destination, Ref: ref}, nil
}

func (p Provider) CreateBranch(ctx context.Context, req contracts.BranchRequest) (contracts.BranchResult, error) {
	if req.Repository == "" || req.Name == "" {
		return contracts.BranchResult{}, errors.New("repository and branch name are required")
	}
	args := []string{"switch", "-c", req.Name}
	if req.From != "" {
		args = append(args, req.From)
	}
	if _, err := p.runner.Run(ctx, req.Repository, args...); err != nil {
		return contracts.BranchResult{}, err
	}
	sha, err := p.runner.Run(ctx, req.Repository, "rev-parse", "HEAD")
	if err != nil {
		return contracts.BranchResult{}, err
	}
	return contracts.BranchResult{Name: req.Name, SHA: strings.TrimSpace(sha)}, nil
}

func (p Provider) Status(ctx context.Context, repository string) (string, error) {
	return p.runner.Run(ctx, repository, "status", "--short")
}

func (p Provider) Diff(ctx context.Context, repository string) (string, error) {
	return p.runner.Run(ctx, repository, "diff", "--no-ext-diff", "--no-color")
}

func (p Provider) Commit(ctx context.Context, repository, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("commit message is required")
	}
	if _, err := p.runner.Run(ctx, repository, "add", "--all"); err != nil {
		return "", err
	}
	return p.runner.Run(ctx, repository, "commit", "-m", message)
}

func (p Provider) Push(ctx context.Context, repository, remote, branch string) error {
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		return errors.New("branch is required")
	}
	_, err := p.runner.Run(ctx, repository, "push", remote, branch)
	return err
}

func (Provider) OpenPR(context.Context, contracts.PullRequest) (contracts.PullRequestResult, error) {
	return contracts.PullRequestResult{}, ErrUnsupported
}
func (Provider) Merge(context.Context, contracts.MergeRequest) error     { return ErrUnsupported }
func (Provider) Comment(context.Context, contracts.CommentRequest) error { return ErrUnsupported }
