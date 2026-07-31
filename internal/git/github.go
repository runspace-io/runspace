package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/runspace/runspace/internal/contracts"
)

type GitHubRemote struct {
	BaseURL, Token string
	Client         *http.Client
}

func (g GitHubRemote) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return http.DefaultClient
}
func (g GitHubRemote) baseURL() string {
	if strings.TrimSpace(g.BaseURL) == "" {
		return "https://api.github.com"
	}
	return strings.TrimRight(g.BaseURL, "/")
}
func (g GitHubRemote) request(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL()+path, &b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := g.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("github request failed: %s", res.Status)
	}
	return out, nil
}
func (g GitHubRemote) ValidateAccess(ctx context.Context, repo string) error {
	if repo == "" {
		return errors.New("repository required")
	}
	_, err := g.request(ctx, http.MethodGet, "/repos/"+repo, nil)
	return err
}
func (g GitHubRemote) OpenPR(ctx context.Context, p contracts.PullRequest) (contracts.PullRequestResult, error) {
	if p.Repository == "" || p.Head == "" || p.Base == "" {
		return contracts.PullRequestResult{}, errors.New("repository, head, and base required")
	}
	out, err := g.request(ctx, http.MethodPost, "/repos/"+p.Repository+"/pulls", map[string]string{"title": p.Title, "body": p.Body, "head": p.Head, "base": p.Base})
	if err != nil {
		return contracts.PullRequestResult{}, err
	}
	n, ok := out["number"].(float64)
	if !ok {
		return contracts.PullRequestResult{}, errors.New("github response missing PR number")
	}
	u, _ := out["html_url"].(string)
	return contracts.PullRequestResult{Number: int(n), URL: u}, nil
}
func (g GitHubRemote) Clone(context.Context, contracts.CloneRequest) (contracts.CloneResult, error) {
	return contracts.CloneResult{}, ErrUnsupported
}
func (g GitHubRemote) CreateBranch(context.Context, contracts.BranchRequest) (contracts.BranchResult, error) {
	return contracts.BranchResult{}, ErrUnsupported
}
func (g GitHubRemote) Merge(context.Context, contracts.MergeRequest) error     { return ErrUnsupported }
func (g GitHubRemote) Comment(context.Context, contracts.CommentRequest) error { return ErrUnsupported }
