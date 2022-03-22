package vcsclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/jfrog/froggit-go/vcsutils"
)

// CommitStatus the status of the commit in the VCS
type CommitStatus int

const (
	// Pass means that the commit passed the tests
	Pass CommitStatus = iota
	// Fail means that the commit failed the tests
	Fail
	// Error means that an unexpected error occured
	Error
	// InProgress means than the status check is in progress
	InProgress
)

// Permission the ssh key permission on the VCS repository
type Permission int

const (
	// Read permission
	Read Permission = iota
	// ReadWrite is either read and write permission
	ReadWrite
)

// VcsInfo is the connection details of the VcsClient to communicate with the server
type VcsInfo struct {
	APIEndpoint string
	Username    string
	Token       string
}

// VcsClient is a base class of all Vcs clients - GitHub, GitLab, Bitbucket server and cloud clients
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
	// payloadURL    - URL to send the payload when a webhook event occurs
	// webhookEvents - The event type
	// Return the webhook ID, token and an error, if occurred
	CreateWebhook(ctx context.Context, owner, repository, branch, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error)

	// UpdateWebhook Update a webhook
	// owner         - User or organization
	// repository    - VCS repository name
	// branch        - VCS branch name
	// payloadURL    - URL to send the payload when a webhook event occurs
	// token         - A token used to validate identity of the incoming webhook
	// webhookID     - The webhook ID returned from a previous CreateWebhook command
	// webhookEvents - The event type
	UpdateWebhook(ctx context.Context, owner, repository, branch, payloadURL, token, webhookID string, webhookEvents ...vcsutils.WebhookEvent) error

	// DeleteWebhook Delete a webhook
	// owner        - User or organization
	// repository   - VCS repository name
	// webhookID    - The webhook ID returned from a previous CreateWebhook command
	DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error

	// SetCommitStatus Set commit status
	// commitStatus - One of Pass, Fail, Error, or InProgress
	// owner        - User or organization
	// repository   - VCS repository name
	// ref          - SHA, a branch name, or a tag name.
	// title        - Title of the commit status
	// description  - Description of the commit status
	// detailsUrl   - The URL for component status link
	SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref, title, description, detailsURL string) error

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
	// owner      - User or organization
	// repository - VCS repository name
	// branch     - The name of the branch
	GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error)

	// AddSshKeyToRepository Add a public ssh key to a repository
	// owner      - User or organization
	// repository - VCS repository name
	// keyName    - Name of the key
	// publicKey  - SSH public key
	// permission - Access permission of the key: read or readWrite
	AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission Permission) error

	// GetRepositoryInfo Returns information about repository.
	// owner      - User or organization
	// repository - VCS repository name
	GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error)

	// GetCommitBySha Get the commit by its SHA
	// owner      - User or organization
	// repository - VCS repository name
	// sha        - The commit hash
	GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error)

	// CreateLabel create a label in repository
	// owner      - User or organization
	// repository - VCS repository name
	// labelInfo  - The label info
	CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error

	// GetLabel get a label related to a repository. Returns (nil, nil) if label doesn't exist.
	// owner      - User or organization
	// repository - VCS repository name
	// name       - Label name
	GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error)

	// UnlabelPullRequest remove a label from a pull request
	// owner         - User or organization
	// repository    - VCS repository name
	// name          - Label name
	// pullRequestID - Pull request ID
	UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error
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

// RepositoryInfo contains general information about repository.
type RepositoryInfo struct {
	CloneInfo CloneInfo
}

// CloneInfo contains URLs that can be used to clone the repository.
type CloneInfo struct {
	// HTTP is a URL string to clone repository using HTTP(S)) protocol.
	HTTP string
	// SSH is a URL string to clone repository using SSH protocol.
	SSH string
}

// LabelInfo contains a label information
type LabelInfo struct {
	Name        string
	Description string
	// Label color is a hexadecimal color code, for example: 4AB548
	Color string
}

func validateParametersNotBlank(paramNameValueMap map[string]string) error {
	errorMessages := make([]string, 0)
	for k, v := range paramNameValueMap {
		if strings.TrimSpace(v) == "" {
			errorMessages = append(errorMessages, fmt.Sprintf("required parameter '%s' is missing", k))
		}
	}
	if len(errorMessages) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errorMessages, ", "))
	}
	return nil
}
