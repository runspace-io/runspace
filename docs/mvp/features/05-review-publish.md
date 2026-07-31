# Feature 5: Review and Publish

## Outcome

The user reviews the agent's actual repository changes and deliberately turns
them into a branch, commit, push, and GitHub pull request.

## Build

- Changed-file summary derived from authoritative Git status.
- Monaco diff viewer with rename, deletion, binary, and size-limit states.
- Publish dialog for validated branch, commit message, PR title/body, and base.
- Durable, idempotent Git publish state machine.
- Native Git branch/commit/push operations with safe arguments and timeouts.
- GitHub `OpenPullRequest` implementation.
- Progress UI, retryable stage errors, and final PR card/link.
- Commit/push/PR events and audit timeline entries.

The MVP publish endpoint is `POST /api/v1/workspaces/{workspaceID}/runs/{runID}/publish`.
It accepts a repository checkout path, validated branch/base, commit metadata,
and PR metadata. Requests are idempotent by run ID and require workspace write
authorization. Configure `GITHUB_TOKEN` for the GitHub PR stage.

## Product rules

- The review UI uses Git truth from the checkout, not agent claims.
- Publishing is an explicit human action.
- Branch names are validated and collisions are reported before mutation.
- The commit contains the full current checkout diff in the MVP; partial staging
  is deferred.
- Empty changes cannot be published.
- Retrying after a partial failure resumes from observed Git/provider state.
- Force push and merge are unavailable.

## Acceptance scenarios

1. Added, modified, deleted, renamed, binary, and large files render correctly.
2. An empty run explains that there is nothing to publish.
3. Valid changes produce a remote branch, one commit, and one GitHub PR.
4. Repeating the publish request does not create another commit or PR.
5. Branch collision, commit-hook failure, push rejection, revoked credentials,
   and provider outage identify the failed stage.
6. The resulting PR URL and commit SHA appear in the timeline and survive
   refresh.

## Not included

Partial staging, inline review comments, merge, force push, PR review requests,
CI control, deployment, or GitLab/Gitea.
