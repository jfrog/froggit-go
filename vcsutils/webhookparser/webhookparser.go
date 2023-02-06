package webhookparser

import (
	"net/http"

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
	// URL to see git comparison (Push event only)
	CompareUrl string `json:"compare_url,omitempty"`
}

// WebHookInfoRepoDetails represents repository info of an incoming webhook
type WebHookInfoRepoDetails struct {
	Name  string `json:"name,omitempty"`
	Owner string `json:"owner,omitempty"`
}

// WebHookInfoCommit represents a commit info of an incoming webhook
type WebHookInfoCommit struct {
	Hash    string `json:"hash,omitempty"`
	Message string `json:"message,omitempty"`
	Url     string `json:"url,omitempty"`
}

type WebHookInfoBranchStatus string

const (
	WebhookinfobranchstatusCreated WebHookInfoBranchStatus = "created"
	WebhookinfobranchstatusUpdated WebHookInfoBranchStatus = "updated"
	WebhookinfobranchstatusDeleted WebHookInfoBranchStatus = "deleted"
)

func branchStatus(existedBefore, existsAfter bool) WebHookInfoBranchStatus {
	switch {
	case existsAfter && !existedBefore:
		return WebhookinfobranchstatusCreated
	case !existsAfter && existedBefore:
		return WebhookinfobranchstatusDeleted
	default:
		return WebhookinfobranchstatusUpdated
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

// WebhookParser is a webhook parser of an incoming webhook from a VCS server
type WebhookParser interface {
	// Validate the webhook payload with the expected token and return the payload
	validatePayload(token []byte) ([]byte, error)
	// Parse the webhook payload and return WebhookInfo
	parseIncomingWebhook(payload []byte) (*WebhookInfo, error)
	// Parse validates and parses the webhook payload.
	Parse(token []byte) (*WebhookInfo, error)
}

// ParseIncomingWebhook parses incoming webhook payload request into a structurized WebhookInfo object.
// See ParserBuilder to get more options.
// provider - The VCS provider
// token    - Token to authenticate incoming webhooks. If empty, signature will not be verified.
// request  - The HTTP request of the incoming webhook
// This is a shortcut for using ParserBuilder.
func ParseIncomingWebhook(provider vcsutils.VcsProvider, token []byte, request *http.Request) (*WebhookInfo, error) {
	parser := NewParserBuilder(provider, request).Build()
	return parser.Parse(token)
}

func validateAndParseHttpRequest(parser WebhookParser, token []byte, request *http.Request) (webhookInfo *WebhookInfo, err error) {
	if request.Body != nil {
		defer func() {
			e := request.Body.Close()
			if e != nil {
				err = e
			}
		}()
	}

	payload, err := parser.validatePayload(token)
	if err != nil {
		return
	}

	webhookInfo, err = parser.parseIncomingWebhook(payload)
	return
}
