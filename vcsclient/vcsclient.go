package vcsclient

import (
	"context"
	"github.com/jfrog/froggit-go/vcsutils"
)

// VcsClient is a base class of all Vcs clients - GitHub, GitLab, Bitbucket server and cloud clients
type VcsClient interface {
	// TestConnection Returns nil if connection and authorization established successfully
	TestConnection(ctx context.Context) error

	// ListRepositories Returns a map between all accessible owners to their list of repositories
	ListRepositories(ctx context.Context) (map[string][]string, error)

	// ListBranches Lists all branches under the input repository
	// owner      - User or organization
	// repository - VCS repository name
	ListBranches(ctx context.Context, owner, repository string) ([]string, error)

	// CreateWebhook Creates a webhook
	// owner         - User or organization
	// repository    - VCS repository name
	// branch        - VCS branch name
	// payloadURL    - URL to send the payload when a webhook event occurs
	// webhookEvents - The event type
	// Return the webhook ID, token and an error, if occurred
	CreateWebhook(ctx context.Context, owner, repository, branch, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error)

	// UpdateWebhook Updates a webhook
	// owner         - User or organization
	// repository    - VCS repository name
	// branch        - VCS branch name
	// payloadURL    - URL to send the payload when a webhook event occurs
	// token         - A token used to validate identity of the incoming webhook
	// webhookID     - The webhook ID returned from a previous CreateWebhook command
	// webhookEvents - The event type
	UpdateWebhook(ctx context.Context, owner, repository, branch, payloadURL, token, webhookID string, webhookEvents ...vcsutils.WebhookEvent) error

	// DeleteWebhook Deletes a webhook
	// owner        - User or organization
	// repository   - VCS repository name
	// webhookID    - The webhook ID returned from a previous CreateWebhook command
	DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error

	// SetCommitStatus Sets commit status
	// commitStatus - One of Pass, Fail, Error, or InProgress
	// owner        - User or organization
	// repository   - VCS repository name
	// ref          - SHA, a branch name, or a tag name.
	// title        - Title of the commit status
	// description  - Description of the commit status
	// detailsUrl   - The URL for component status link
	SetCommitStatus(ctx context.Context, commitStatus vcsutils.CommitStatus, owner, repository, ref, title, description, detailsURL string) error

	// GetCommitStatuses Gets all statuses for a specific commit
	// owner        - User or organization
	// repository   - VCS repository name
	// ref          - SHA, a branch name, or a tag name.
	GetCommitStatuses(ctx context.Context, owner, repository, ref string) (status []vcsutils.CommitStatusInfo, err error)

	// DownloadRepository Downloads and extracts a VCS repository
	// owner      - User or organization
	// repository - VCS repository name
	// branch     - VCS branch name
	// localPath  - Local file system path
	DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error

	// CreatePullRequest Creates a pull request between 2 different branches in the same repository
	// owner        - User or organization
	// repository   - VCS repository name
	// sourceBranch - Source branch
	// targetBranch - Target branch
	// title        - Pull request title
	// description  - Pull request description
	CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) error

	// AddPullRequestComment Adds a new comment on the requested pull request
	// owner          - User or organization
	// repository     - VCS repository name
	// content        - The new comment content
	// pullRequestID  - Pull request ID
	AddPullRequestComment(ctx context.Context, owner, repository, content string, pullRequestID int) error

	// ListPullRequestComments Gets all comments assigned to a pull request.
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestID  - Pull request ID
	ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]vcsutils.CommentInfo, error)

	// ListOpenPullRequests Gets all open pull requests ids.
	// owner          - User or organization
	// repository     - VCS repository name
	ListOpenPullRequests(ctx context.Context, owner, repository string) ([]vcsutils.PullRequestInfo, error)

	// GetLatestCommit Gets the most recent commit of a branch
	// owner      - User or organization
	// repository - VCS repository name
	// branch     - The name of the branch
	GetLatestCommit(ctx context.Context, owner, repository, branch string) (vcsutils.CommitInfo, error)

	// AddSshKeyToRepository Adds a public ssh key to a repository
	// owner      - User or organization
	// repository - VCS repository name
	// keyName    - Name of the key
	// publicKey  - SSH public key
	// permission - Access permission of the key: read or readWrite
	AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission vcsutils.Permission) error

	// GetRepositoryInfo Returns information about repository.
	// owner      - User or organization
	// repository - VCS repository name
	GetRepositoryInfo(ctx context.Context, owner, repository string) (vcsutils.RepositoryInfo, error)

	// GetCommitBySha Gets the commit by its SHA
	// owner      - User or organization
	// repository - VCS repository name
	// sha        - The commit hash
	GetCommitBySha(ctx context.Context, owner, repository, sha string) (vcsutils.CommitInfo, error)

	// CreateLabel Creates a label in repository
	// owner      - User or organization
	// repository - VCS repository name
	// labelInfo  - The label info
	CreateLabel(ctx context.Context, owner, repository string, labelInfo vcsutils.LabelInfo) error

	// GetLabel Gets a label related to a repository. Returns (nil, nil) if label doesn't exist.
	// owner      - User or organization
	// repository - VCS repository name
	// name       - Label name
	GetLabel(ctx context.Context, owner, repository, name string) (*vcsutils.LabelInfo, error)

	// ListPullRequestLabels Gets all labels assigned to a pull request.
	// owner         - User or organization
	// repository    - VCS repository name
	// pullRequestID - Pull request ID
	ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error)

	// UnlabelPullRequest Removes a label from a pull request
	// owner         - User or organization
	// repository    - VCS repository name
	// name          - Label name
	// pullRequestID - Pull request ID
	UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error

	// UploadCodeScanning Upload Scanning Analysis uploads a scanning analysis file to the relevant git provider
	// owner         - User or organization
	// repository    - VCS repository name
	// branch        - The name of the branch
	// scan          - Code scanning analysis
	UploadCodeScanning(ctx context.Context, owner, repository, branch, scanResults string) (string, error)

	// DownloadFileFromRepo Downloads a file from path in a repository
	// owner         - User or organization
	// repository    - VCS repository name
	// branch        - The name of the branch
	// path          - The path to the requested file
	DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) ([]byte, int, error)

	// GetRepositoryEnvironmentInfo Gets the environment info configured for a repository
	// owner         - User or organization
	// repository    - VCS repository name
	// name          - The environment name
	GetRepositoryEnvironmentInfo(ctx context.Context, owner, repository, name string) (vcsutils.RepositoryEnvironmentInfo, error)

	// GetModifiedFiles returns list of file names modified between two VCS references
	// owner         - User or organization
	// repository    - VCS repository name
	// refBefore     - A VCS reference: commit SHA, branch name, tag name
	// refAfter      - A VCS reference: commit SHA, branch name, tag name
	GetModifiedFiles(ctx context.Context, owner, repository, refBefore, refAfter string) ([]string, error)
}
