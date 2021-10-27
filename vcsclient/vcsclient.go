package vcsclient

import (
	"context"
	"fmt"
	"github.com/jfrog/froggit-go/vcsutils"
	"strings"
)

type CommitStatus int

const (
	Pass CommitStatus = iota
	Fail
	Error
	InProgress
)

type VcsInfo struct {
	ApiEndpoint string
	Username    string
	Token       string
}

type VcsClient interface {
	// TestConnection Return nil if connection and authorization established successfully
	TestConnection(ctx context.Context) error

	// ListRepositories Return a map between all accessible owners to their list of repositories
	ListRepositories(ctx context.Context) (map[string][]string, error)

	// ListBranches List all branches under the input repository
	// owner      - User or organization
	// repository - VCS repository name
	ListBranches(ctx context.Context, owner, repository string) ([]string, error)

	// CreateWebhook Create a webhook
	// owner         - User or organization
	// repository    - VCS repository name
	// branch        - VCS branch name
	// payloadUrl    - URL to send the payload when a webhook event occurs
	// webhookEvents - PrCreated, PrEdited, or Push
	// Return the webhook ID, token and an error, if occurred
	CreateWebhook(ctx context.Context, owner, repository, branch, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error)

	// UpdateWebhook Update a webhook
	// owner         - User or organization
	// repository    - VCS repository name
	// branch        - VCS branch name
	// payloadUrl    - URL to send the payload when a webhook event occurs
	// token         - A token used to validate identity of the incoming webhook
	// webhookId     - The webhook ID returned from a previous CreateWebhook command
	// webhookEvents - PrCreated, PrEdited, or Push
	UpdateWebhook(ctx context.Context, owner, repository, branch, payloadUrl, token, webhookId string, webhookEvents ...vcsutils.WebhookEvent) error

	// DeleteWebhook Delete a webhook
	// owner        - User or organization
	// repository   - VCS repository name
	// webhookId    - The webhook ID returned from a previous CreateWebhook command
	DeleteWebhook(ctx context.Context, owner, repository, webhookId string) error

	// SetCommitStatus Set commit status
	// commitStatus - One of Pass, Fail, Error, or InProgress
	// owner        - User or organization
	// repository   - VCS repository name
	// ref          - SHA, a branch name, or a tag name.
	// title        - Title of the commit status
	// description  - Description of the commit status
	// detailsUrl   - The URL for component status link
	SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref, title, description, detailsUrl string) error

	// DownloadRepository Download and extract a VCS repository
	// owner      - User or organization
	// repository - VCS repository name
	// branch     - VCS branch name
	// localPath  - Local file system path
	DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error

	// CreatePullRequest Create a pull request between 2 different branches in the same repository
	// owner        - User or organization
	// repository   - VCS repository name
	// sourceBranch - Source branch
	// targetBranch - Target branch
	// title        - Pull request title
	// description  - Pull request description
	CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) error

	// GetLatestCommit Get the most recent commit of a branch
	// owner        - User or organization
	// repository   - VCS repository name
	// branch       - The name of the branch
	GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error)
}

// CommitInfo contains the details of a commit
type CommitInfo struct {
	// The SHA-1 hash of the commit
	Hash string
	// The author's name
	AuthorName string
	// The committer's name
	CommitterName string
	// The commit URL
	Url string
	// Seconds from epoch
	Timestamp int64
	// The commit message
	Message string
	// The SHA-1 hashes of the parent commits
	ParentHashes []string
}

func validateParametersNotBlank(parameters ...string) error {
	for _, parameter := range parameters {
		if len(strings.TrimSpace(parameter)) == 0 {
			return fmt.Errorf("required parameter is empty")
		}
	}
	return nil
}
