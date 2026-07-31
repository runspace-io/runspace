package contracts

import "context"

// GitProvider abstracts hosting-provider operations from orchestration logic.
type GitProvider interface {
	Clone(context.Context, CloneRequest) (CloneResult, error)
	CreateBranch(context.Context, BranchRequest) (BranchResult, error)
	OpenPR(context.Context, PullRequest) (PullRequestResult, error)
	Merge(context.Context, MergeRequest) error
	Comment(context.Context, CommentRequest) error
}

type CloneRequest struct {
	RepositoryURL string
	Destination   string
	Ref           string
}

type CloneResult struct {
	Path string
	Ref  string
}

type BranchRequest struct {
	Repository string
	Name       string
	From       string
}

type BranchResult struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

type PullRequest struct {
	Repository string
	Title      string
	Body       string
	Head       string
	Base       string
}

type PullRequestResult struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type MergeRequest struct {
	Repository string
	Number     int
}

type CommentRequest struct {
	Repository string
	Number     int
	Body       string
}
