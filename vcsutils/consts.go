package vcsutils

const (
	branchPrefix           = "refs/heads/"
	TagPrefix              = "refs/tags/"
	NumberOfCommitsToFetch = 50
	ErrNoCommentsProvided  = "could not add a pull request review comment, no comments were provided"
	ErrApiEndpointNotSet   = "the API endpoint must be set for "
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

type PullRequestState string

const (
	Open   PullRequestState = "open"
	Closed PullRequestState = "closed"
)
