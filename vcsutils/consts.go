package vcsutils

import (
	"fmt"
	"strings"
	"time"
)

const (
	branchPrefix = "refs/heads/"
	TagPrefix    = "refs/tags/"
)

// VcsProvider is an enum represents the VCS provider type
type VcsProvider int

const (
	// GitHub VCS provider
	GitHub VcsProvider = iota
	// GitLab VCS provider
	GitLab
	// BitbucketServer VCS provider
	BitbucketServer
	// BitbucketCloud VCS provider
	BitbucketCloud
	// AzureRepos VCS provider
	AzureRepos
)

// String representation of the VcsProvider
func (v VcsProvider) String() string {
	switch v {
	case GitHub:
		return "GitHub"
	case GitLab:
		return "GitLab"
	case BitbucketServer:
		return "Bitbucket Server"
	case BitbucketCloud:
		return "Bitbucket Cloud"
	case AzureRepos:
		return "Azure Repos"
	default:
		return ""
	}
}

// WebhookEvent is the event type of the incoming webhook
type WebhookEvent string

const (
	// PrRejected the pull request is rejected
	PrRejected WebhookEvent = "PrRejected"
	// PrEdited the pull request is edited
	PrEdited WebhookEvent = "PrEdited"
	// PrMerged the pull request is merged
	PrMerged WebhookEvent = "PrMerged"
	// PrOpened a pull request is opened
	PrOpened WebhookEvent = "PrOpened"
	// Push a commit is pushed to the source branch
	Push WebhookEvent = "Push"
	// TagPushed a new tag is pushed
	TagPushed WebhookEvent = "TagPushed"
	// TagRemoved a tag is removed
	TagRemoved WebhookEvent = "TagRemoved"
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

type CommentInfo struct {
	ID      int64
	Content string
	Created time.Time
}

type PullRequestInfo struct {
	ID     int64
	Source BranchInfo
	Target BranchInfo
}

type BranchInfo struct {
	Name       string
	Repository string
}

// RepositoryInfo contains general information about repository.
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

// VcsInfo is the connection details of the VcsClient to communicate with the server
type VcsInfo struct {
	APIEndpoint string
	Username    string
	Token       string
	// Project name is relevant for Azure Repos
	Project string
}
type PullRequestState string

const (
	Open   PullRequestState = "open"
	Closed PullRequestState = "closed"
)

func MapPullRequestState(state *PullRequestState) *string {
	var stateStringValue string
	switch *state {
	case Open:
		stateStringValue = "open"
	case Closed:
		stateStringValue = "closed"
	default:
		return nil
	}
	return &stateStringValue
}

func ValidateParametersNotBlank(paramNameValueMap map[string]string) error {
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
func CommitStatusAsStringToStatus(rawStatus string) CommitStatus {
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

func ExtractTimeWithFallback(timeObject *time.Time) time.Time {
	if timeObject == nil {
		return time.Time{}
	}
	return timeObject.UTC()
}
