package vcsclient

import "github.com/jfrog/froggit-go/vcsutils"

type CommitStatus int

const (
	Pass CommitStatus = iota
	Fail
	Error
	InProgress
)

type TokenPermission int

const (
	WebhookPerm TokenPermission = iota
	CommitStatusPerm
)

type VcsInfo struct {
	ApiEndpoint string
	Username    string
	Token       string
}

type VcsClient interface {
	// Return nil if connection and authorization established successfully
	TestConnection() error

	// List all repositories of the user and the teams that the users belongs
	ListRepositories() (map[string][]string, error)

	// List all branches under the input repository
	// owner      - User or organiaztion
	// repository - VCS repository name
	ListBranches(owner, repository string) ([]string, error)

	// Create a webhook
	// owner         - User or organiaztion
	// repository    - VCS repository name
	// branch 	     - VCS branch name
	// payloadUrl    - URL to send the payload when a webhook event occurs
	// webhookEvents - PrCreated, PrEdited, or Push
	// Return the webhook ID, token and an error, if occurred
	CreateWebhook(owner, repository, branch, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error)

	// Update a webhook
	// owner         - User or organiaztion
	// repository    - VCS repository name
	// branch 	     - VCS branch name
	// payloadUrl    - URL to send the payload when a webhook event occurs
	// token         - A token used to validate identity of the incoming webhook
	// webhookId     - The webhook ID returned from a previous CreateWebhook command
	// webhookEvents - PrCreated, PrEdited, or Push
	UpdateWebhook(owner, repository, branch, payloadUrl, token, webhookId string, webhookEvents ...vcsutils.WebhookEvent) error

	// Delete a webhook
	// owner        - User or organiaztion
	// repository   - VCS repository name
	// webhookId    - The webhook ID returned from a previous CreateWebhook command
	DeleteWebhook(owner, repository, webhookId string) error

	// Set commit status
	// commitStatus - One of Pass, Fail, Error, or InProgress
	// owner        - User or organiaztion
	// repository   - VCS repository name
	// ref          - SHA, a branch name, or a tag name.
	// title        - Title of the commit status
	// description  - Description of the commit status
	// detailsUrl   - URL leads to the platform to provide more information, such as Xray scanning results
	SetCommitStatus(commitStatus CommitStatus, owner, repository, ref, title, description, detailsUrl string) error

	// Download and extract a VCS repository
	// owner      - User or organiaztion
	// repository - VCS repository name
	// branch 	  - VCS branch name
	// localPath  - Local file system path
	DownloadRepository(owner, repository, branch, localPath string) error

	// Create a pull request between 2 different branched in the same repository
	// owner        - User or organiaztion
	// repository   - VCS repository name
	// sourceBranch - Source branch
	// targetBranch - Target branch
	// title        - Pull request title
	// description  - Pull request description
	CreatePullRequest(owner, repository, sourceBranch, targetBranch, title, description string) error
}
