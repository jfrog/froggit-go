package webhookparser

import (
	"context"
	"net/http"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

const (
	// EventHeaderKey represents the event type of incoming webhook from Bitbucket
	EventHeaderKey = "X-Event-Key"
	// gitNilHash is the hash value used by Git to indicate a non-existent commit.
	gitNilHash = "0000000000000000000000000000000000000000"
)

// WebhookInfo used for parsing an incoming webhook request from the VCS provider.
type WebhookInfo struct {
	// The target repository for pull requests and push
	TargetRepositoryDetails WebHookInfoRepoDetails `json:"target_repository_details,omitempty"`
	// The target branch for pull requests and push
	TargetBranch string `json:"branch,omitempty"`
	// Pull request id
	PullRequestId int `json:"pull_request_id,omitempty"`
	// The source repository for pull requests
	SourceRepositoryDetails WebHookInfoRepoDetails `json:"source_repository_details,omitempty"`
	// The source branch for pull requests
	SourceBranch string `json:"source_branch,omitempty"`
	// Seconds from epoch
	Timestamp int64 `json:"timestamp,omitempty"`
	// The event type
	Event vcsutils.WebhookEvent `json:"event,omitempty"`
	// Last commit (Push event only)
	Commit WebHookInfoCommit `json:"commit,omitempty"`
	// Before commit (Push event only)
	BeforeCommit WebHookInfoCommit `json:"before_commit,omitempty"`
	// Branch status (Push event only)
	BranchStatus WebHookInfoBranchStatus `json:"branch_status,omitempty"`
	// User who triggered the commit (Push event only)
	TriggeredBy WebHookInfoUser `json:"triggered_by,omitempty"`
	// Committer (Push event only)
	Committer WebHookInfoUser `json:"committer,omitempty"`
	// Commit author (Push event only)
	Author WebHookInfoUser `json:"author,omitempty"`
	// CompareUrl is HTML URL to see git comparison between commits (Push event only)
	CompareUrl string `json:"compare_url,omitempty"`
	// PullRequest encapsulates information of the pull request.
	PullRequest *WebhookInfoPullRequest `json:"pull_request,omitempty"`
	// Tag encapsulates information about the tag event.
	Tag *WebhookInfoTag `json:"tag,omitempty"`
}

// WebhookInfoPullRequest contains information about a pull request event received via a webhook.
type WebhookInfoPullRequest struct {
	// ID is a unique identifier of the pull request.
	ID int `json:"id,omitempty"`
	// Title is a title(name) of the pull request.
	Title string `json:"title,omitempty"`
	// CompareUrl is a hyperlink to the pull request.
	CompareUrl string `json:"url,omitempty"`
	// Timestamp of the last update (Unix timestamp).
	Timestamp int64 `json:"timestamp,omitempty"`
	// Author is an info about pull request author.
	Author WebHookInfoUser `json:"author,omitempty"`
	// TriggeredBy
	TriggeredBy WebHookInfoUser `json:"triggered_by,omitempty"`
	// SkipDecryption
	SkipDecryption bool `json:"skip_decryption,omitempty"`
	// TargetRepository contains details about target repository (destination of the changes).
	TargetRepository WebHookInfoRepoDetails `json:"target_repository,omitempty"`
	// TargetBranch is a name of the branch of the TargetRepository.
	TargetBranch string `json:"target_branch,omitempty"`
	// TargetHash is a commit SHA of the target branch.
	TargetHash string `json:"target_hash,omitempty"`
	// SourceRepository contains details about source repository.
	SourceRepository WebHookInfoRepoDetails `json:"source_repository,omitempty"`
	// SourceBranch is a name of the branch of the SourceRepository.
	SourceBranch string `json:"source_branch,omitempty"`
	// SourceHash is a commit SHA of the source branch.
	SourceHash string `json:"source_hash,omitempty"`
}

// WebHookInfoRepoDetails represents repository info of an incoming webhook
type WebHookInfoRepoDetails struct {
	Name  string `json:"name,omitempty"`
	Owner string `json:"owner,omitempty"`
}

// WebhookInfoTag contains information about a tag event received via a webhook.
type WebhookInfoTag struct {
	// Name is a name of the tag.
	Name string `json:"name,omitempty"`
	// Hash is a SHA of the tag.
	Hash string `json:"hash,omitempty"`
	// TargetHash is a SHA of the commit the tag points to.
	TargetHash string `json:"target_hash,omitempty"`
	// Message is a message used during tag creation if any.
	Message string `json:"message,omitempty"`
	// Repository contains details about repository.
	Repository WebHookInfoRepoDetails `json:"repository,omitempty"`
	// Author
	Author WebHookInfoUser `json:"author,omitempty"`
}

// WebHookInfoCommit represents a commit info of an incoming webhook
type WebHookInfoCommit struct {
	Hash    string `json:"hash,omitempty"`
	Message string `json:"message,omitempty"`
	Url     string `json:"url,omitempty"`
}

type WebHookInfoBranchStatus string

const (
	WebhookInfoBranchStatusCreated WebHookInfoBranchStatus = "created"
	WebhookInfoBranchStatusUpdated WebHookInfoBranchStatus = "updated"
	WebhookInfoBranchStatusDeleted WebHookInfoBranchStatus = "deleted"
)

func branchStatus(existedBefore, existsAfter bool) WebHookInfoBranchStatus {
	switch {
	case existsAfter && !existedBefore:
		return WebhookInfoBranchStatusCreated
	case !existsAfter && existedBefore:
		return WebhookInfoBranchStatusDeleted
	default:
		return WebhookInfoBranchStatusUpdated
	}
}

type WebHookInfoUser struct {
	Login       string `json:"login,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	AvatarUrl   string `json:"avatar_url,omitempty"`
}

type WebHookInfoFile struct {
	Path string `json:"path,omitempty"`
}

// webhookParser is a webhook parser of an incoming webhook from a VCS server
type webhookParser interface {
	// Validate the webhook payload with the expected token and return the payload
	validatePayload(ctx context.Context, request *http.Request, token []byte) ([]byte, error)
	// Parse the webhook payload and return WebhookInfo
	parseIncomingWebhook(ctx context.Context, request *http.Request, payload []byte) (*WebhookInfo, error)
}

// ParseIncomingWebhook parses incoming webhook HTTP request into WebhookInfo struct.
// ctx - Go context
// logger - Used to log any trace about the parsing
// origin - Information about the hook origin
// request - Received HTTP request
func ParseIncomingWebhook(ctx context.Context, logger vcsclient.Log, origin WebhookOrigin, request *http.Request) (*WebhookInfo, error) {
	parser := createWebhookParser(logger, origin)
	return validateAndParseHttpRequest(ctx, logger, parser, origin.Token, request)
}

// WebhookOrigin provides information about the hook to parse.
type WebhookOrigin struct {
	// Git provider
	VcsProvider vcsutils.VcsProvider
	// URL of the Git service
	OriginURL string
	// Token is used to authenticate incoming webhooks. If empty, signature will not be verified.
	// The token is a random key generated in the CreateWebhook command.
	Token []byte
}

func validateAndParseHttpRequest(ctx context.Context, logger vcsclient.Log, parser webhookParser, token []byte, request *http.Request) (*WebhookInfo, error) {
	if request.Body != nil {
		defer func() {
			err := request.Body.Close()
			if err != nil {
				logger.Warn("Error when closing HTTP request body: ", err)
			}
		}()
	}

	payload, err := parser.validatePayload(ctx, request, token)
	if err != nil {
		return nil, err
	}

	return parser.parseIncomingWebhook(ctx, request, payload)
}
