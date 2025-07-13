package vcsclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jfrog/froggit-go/vcsutils"
)

// CommitStatus the status of the commit in the VCS
type CommitStatus int

const (
	// Pass means that the commit passed the tests
	Pass CommitStatus = iota
	// Fail means that the commit failed the tests
	Fail
	// Error means that an unexpected error occurred
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

// RepositoryVisibility the visibility level of the repository
type RepositoryVisibility int

const (
	// Open to public
	Public RepositoryVisibility = iota
	// Open to organization
	Internal
	// Open to user
	Private
)

// VcsInfo is the connection details of the VcsClient to communicate with the server
type VcsInfo struct {
	APIEndpoint string
	Username    string
	Token       string
	// Project name is relevant for Azure Repos
	Project string
}

// RepositoryEnvironmentInfo is the environment details configured for a repository
type RepositoryEnvironmentInfo struct {
	Name      string
	Url       string
	Reviewers []string
}

// CommitStatusInfo status which is then reflected in pull requests involving those commits
// State         - One of success, pending, failure, or error
// Description   - Description of the commit status
// DetailsUrl    - The URL for component status link
// Creator       - The creator of the status
// CreatedAt     - Date of status creation
// LastUpdatedAt - Date of status last update time.
type CommitStatusInfo struct {
	State         CommitStatus
	Description   string
	DetailsUrl    string
	Creator       string
	CreatedAt     time.Time
	LastUpdatedAt time.Time
}

// AppRepositoryInfo contains information about an application repository
// ID            - The unique identifier of the repository
// Name          - The repository name
// FullName      - The full name of the repository (including owner/namespace)
// Owner         - The owner of the repository
// Private       - Whether the repository is private
// Description   - The repository description
// URL           - The web URL of the repository
// CloneURL      - The HTTP(S) clone URL of the repository
// SSHURL        - The SSH clone URL of the repository
// DefaultBranch - The default branch of the repository
type AppRepositoryInfo struct {
	ID            int64
	Name          string
	FullName      string
	Owner         string
	Private       bool
	Description   string
	URL           string
	CloneURL      string
	SSHURL        string
	DefaultBranch string
}

// CreatedPullRequestInfo contains the data returned from a pull request creation
// Number - The number of the pull request
// URL - The URL of the pull request
// StatusesUrl - The URL to the commit statuses of the pull request
type CreatedPullRequestInfo struct {
	Number      int
	URL         string
	StatusesUrl string
}

// VcsClient is a base class of all Vcs clients - GitHub, GitLab, Bitbucket server and cloud clients
type VcsClient interface {
	// TestConnection Returns nil if connection and authorization established successfully
	TestConnection(ctx context.Context) error

	// ListRepositories Returns a map between all accessible owners to their list of repositories
	ListRepositories(ctx context.Context) (map[string][]string, error)

	// ListAppRepositories ListRepositories Returns a map between all accessible App to their list of repositories
	ListAppRepositories(ctx context.Context) ([]AppRepositoryInfo, error)

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
	SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref, title, description, detailsURL string) error

	// GetCommitStatuses Gets all statuses for a specific commit
	// owner        - User or organization
	// repository   - VCS repository name
	// ref          - SHA, a branch name, or a tag name.
	GetCommitStatuses(ctx context.Context, owner, repository, ref string) (status []CommitStatusInfo, err error)

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

	// CreatePullRequestDetailed Creates a pull request between 2 different branches in the same repository
	// owner        - User or organization
	// repository   - VCS repository name
	// sourceBranch - Source branch
	// targetBranch - Target branch
	// title        - Pull request title
	// description  - Pull request description
	CreatePullRequestDetailed(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) (CreatedPullRequestInfo, error)

	// UpdatePullRequest Updates pull requests metadata
	// owner        		    - User or organization
	// repository    		    - VCS repository name
	// title         	        - Pull request title
	// body                     - Pull request body or description
	// targetBranchName         - Name of the pull request target branch name,For non-change, pass an empty string.
	// prId				        - Pull request ID
	// state				    - Pull request state
	UpdatePullRequest(ctx context.Context, owner, repository, title, body, targetBranchName string, prId int, state vcsutils.PullRequestState) error

	// AddPullRequestComment Adds a new comment on the requested pull request
	// owner          - User or organization
	// repository     - VCS repository name
	// content        - The new comment content
	// pullRequestID  - Pull request ID
	AddPullRequestComment(ctx context.Context, owner, repository, content string, pullRequestID int) error

	// AddPullRequestReviewComments Adds a new review comment on the requested pull request
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestID  - Pull request ID
	// comment        - The new comment details defined in PullRequestComment
	AddPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...PullRequestComment) error

	// ListPullRequestReviews List all reviews assigned to a pull request.
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestID  - Pull request ID
	// comment        - The new comment details defined in PullRequestComment
	ListPullRequestReviews(ctx context.Context, owner, repository string, pullRequestID int) ([]PullRequestReviewDetails, error)

	// ListPullRequestReviewComments Gets all pull request review comments
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestID  - Pull request ID
	ListPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error)

	// DeletePullRequestReviewComments Gets all comments assigned to a pull request.
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestID  - Pull request ID
	// commentID 	  - The ID of the comment
	DeletePullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...CommentInfo) error

	// ListPullRequestComments Gets all comments assigned to a pull request.
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestID  - Pull request ID
	ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error)

	// DeletePullRequestComment deleted a specific comment in a pull request.
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestID  - Pull request ID
	// commentID 	  - The ID of the comment
	DeletePullRequestComment(ctx context.Context, owner, repository string, pullRequestID, commentID int) error

	// ListOpenPullRequestsWithBody Gets all open pull requests ids and the pull request body.
	// owner          - User or organization
	// repository     - VCS repository name
	ListOpenPullRequestsWithBody(ctx context.Context, owner, repository string) ([]PullRequestInfo, error)

	// ListOpenPullRequests Gets all open pull requests ids.
	// owner          - User or organization
	// repository     - VCS repository name
	ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error)

	// GetPullRequestByID Gets pull request info by ID.
	// owner          - User or organization
	// repository     - VCS repository name
	// pullRequestId  - ID of the pull request
	GetPullRequestByID(ctx context.Context, owner, repository string, pullRequestId int) (PullRequestInfo, error)

	// GetLatestCommit Gets the most recent commit of a branch
	// owner      - User or organization
	// repository - VCS repository name
	// branch     - The name of the branch
	GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error)

	// GetCommits Gets the most recent commit of a branch
	// owner      - User or organization
	// repository - VCS repository name
	// branch     - The name of the branch
	GetCommits(ctx context.Context, owner, repository, branch string) ([]CommitInfo, error)

	// GetCommitsWithQueryOptions Gets repository commits considering GitCommitsQueryOptions provided by the user.
	// owner       - User or organization
	// repository  - VCS repository name
	// listOptions - Optional parameters for the 'ListCommits' method
	GetCommitsWithQueryOptions(ctx context.Context, owner, repository string, options GitCommitsQueryOptions) ([]CommitInfo, error)

	// ListPullRequestsAssociatedWithCommit Lists pull requests associated with the commit.
	// owner       - User or organization
	// repository  - VCS repository name
	// commitSHA   - commit sha
	ListPullRequestsAssociatedWithCommit(ctx context.Context, owner, repository string, commitSHA string) ([]PullRequestInfo, error)

	// AddSshKeyToRepository Adds a public ssh key to a repository
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

	// GetCommitBySha Gets the commit by its SHA
	// owner      - User or organization
	// repository - VCS repository name
	// sha        - The commit hash
	GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error)

	// CreateLabel Creates a label in repository
	// owner      - User or organization
	// repository - VCS repository name
	// labelInfo  - The label info
	CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error

	// GetLabel Gets a label related to a repository. Returns (nil, nil) if label doesn't exist.
	// owner      - User or organization
	// repository - VCS repository name
	// name       - Label name
	GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error)

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
	GetRepositoryEnvironmentInfo(ctx context.Context, owner, repository, name string) (RepositoryEnvironmentInfo, error)

	// GetModifiedFiles returns list of file names modified between two VCS references
	// owner         - User or organization
	// repository    - VCS repository name
	// refBefore     - A VCS reference: commit SHA, branch name, tag name
	// refAfter      - A VCS reference: commit SHA, branch name, tag name
	GetModifiedFiles(ctx context.Context, owner, repository, refBefore, refAfter string) ([]string, error)

	// GetPullRequestCommentSizeLimit returns the maximum size of a pull request comment
	GetPullRequestCommentSizeLimit() int

	// GetPullRequestDetailsSizeLimit returns the maximum size of a pull request details
	GetPullRequestDetailsSizeLimit() int

	// CreateBranch creates a new branch in the specified repository using the provided source branch as a base.
	CreateBranch(ctx context.Context, owner, repository, sourceBranch, newBranch string) error

	// AllowWorkflows allows the user to enable or disable workflows for an organization
	AllowWorkflows(ctx context.Context, owner string) error

	// AddOrganizationSecret adds a secret to the organization
	AddOrganizationSecret(ctx context.Context, owner, secretName, secretValue string) error

	// CreateOrgVariable creates a variable in the organization
	CreateOrgVariable(ctx context.Context, owner, variableName, variableValue string) error

	// CommitAndPushFiles commits and pushes files to the specified branch in the repository
	CommitAndPushFiles(ctx context.Context, owner, repo, sourceBranch, commitMessage, authorName, authorEmail string, files []FileToCommit) error

	// GetRepoCollaborators returns a list of collaborators for the specified repository
	GetRepoCollaborators(ctx context.Context, owner, repo, affiliation, permission string) ([]string, error)

	// GetRepoTeamsByPermissions returns a list of teams with the proper permissions
	GetRepoTeamsByPermissions(ctx context.Context, owner, repo string, permissions []string) ([]int64, error)

	// CreateOrUpdateEnvironment creates or updates an environment in the specified repository
	CreateOrUpdateEnvironment(ctx context.Context, owner, repo, envName string, teams []int64, users []string) error

	// MergePullRequest merges a pull request into the target branch
	MergePullRequest(ctx context.Context, owner, repo string, prNumber int, commitMessage string) error

	// UploadSnapshotToDependencyGraph uploads a snapshot to the GitHub dependency graph tab
	UploadSnapshotToDependencyGraph(ctx context.Context, snapshot SbomSnapshot) error
}

// SbomSnapshot represents a snapshot for GitHub dependency submission API
type SbomSnapshot struct {
	Version   int                  `json:"version"`
	SHA       string               `json:"sha"`
	Ref       string               `json:"ref"`
	Job       *JobInfo             `json:"job"`
	Detector  *DetectorInfo        `json:"detector"`
	Scanned   string               `json:"scanned"`
	Manifests map[string]*Manifest `json:"manifests"`
}

// JobInfo contains information about the job that created the snapshot
type JobInfo struct {
	Correlator string `json:"correlator"`
	ID         string `json:"id"`
}

// DetectorInfo contains information about the detector that created the snapshot
type DetectorInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Manifest represents a manifest file with its dependencies
type Manifest struct {
	Name     string                         `json:"name"`
	File     *FileInfo                      `json:"file"`
	Resolved map[string]*ResolvedDependency `json:"resolved"`
}

// FileInfo contains information about the manifest file
type FileInfo struct {
	SourceLocation string `json:"source_location"`
}

// ResolvedDependency represents a resolved dependency with its package URL and dependencies
type ResolvedDependency struct {
	PackageURL   string   `json:"package_url"`
	Relationship string   `json:"relationship"`
	Dependencies []string `json:"dependencies,omitempty"`
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
	// The email of the commit author
	AuthorEmail string
}

type CommentInfo struct {
	ID       int64
	ThreadID string
	Content  string
	Created  time.Time
	Version  int
}

type PullRequestInfo struct {
	ID     int64
	Title  string
	Body   string
	URL    string
	Author string
	Source BranchInfo
	Target BranchInfo
	Status string
}

type PullRequestReviewDetails struct {
	ID          int64
	Reviewer    string
	Body        string
	SubmittedAt string
	CommitID    string
	State       string
}

type BranchInfo struct {
	Name       string
	Repository string
	Owner      string
}

// PullRequestInfo contains the details of a pull request comment
// content - the content of the pull request comment
// PullRequestDiff - the content of the pull request diff
type PullRequestComment struct {
	CommentInfo
	PullRequestDiff
}

// PullRequestDiff contains the details of the pull request diff
// OriginalFilePath   - the original file path
// OriginalStartLine  - the original start line number
// OriginalEndLine    - the original end line number
// originalStartColum - the original start column number
// OriginalEndColumn  - the original end column number
// NewFilePath        - the new file path
// NewStartLine       - the new start line number
// NewEndLine         - the new end line number
// NewStartColumn     - the new start column number
// NewEndColumn       - the new end column number
type PullRequestDiff struct {
	OriginalFilePath    string
	OriginalStartLine   int
	OriginalEndLine     int
	OriginalStartColumn int
	OriginalEndColumn   int
	NewFilePath         string
	NewStartLine        int
	NewEndLine          int
	NewStartColumn      int
	NewEndColumn        int
}

// RepositoryInfo contains general information about the repository.
type RepositoryInfo struct {
	CloneInfo            CloneInfo
	RepositoryVisibility RepositoryVisibility
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

// GitCommitsQueryOptions specifies the optional parameters fot the commit list.
type GitCommitsQueryOptions struct {
	// Since when should Commits be included in the response.
	Since time.Time
	ListOptions
}

// ListOptions specifies the optional parameters to various List methods that support offset pagination.
type ListOptions struct {
	// For paginated result sets, page of results to retrieve.
	Page int
	// For paginated result sets, the number of results to include per page.
	PerPage int
}

type FileToCommit struct {
	Path    string
	Content string
}

func validateParametersNotBlank(paramNameValueMap map[string]string) error {
	var errorMessages []string
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

// commitStatusAsStringToStatus maps status as string to CommitStatus
// Handles all the different statuses for every VCS provider
func commitStatusAsStringToStatus(rawStatus string) CommitStatus {
	switch strings.ToLower(rawStatus) {
	case "success", "succeeded", "successful":
		return Pass
	case "fail", "failure", "failed":
		return Fail
	case "pending", "inprogress":
		return InProgress
	default:
		return Error
	}
}

func extractTimeWithFallback(timeObject *time.Time) time.Time {
	if timeObject == nil {
		return time.Time{}
	}
	return timeObject.UTC()
}
