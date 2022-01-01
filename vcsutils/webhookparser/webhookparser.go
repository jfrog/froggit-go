package webhookparser

import (
	"net/http"

	"github.com/jfrog/froggit-go/vcsutils"
)

// EventHeaderKey represents the event type of an incoming webhook from Bitbucket
const EventHeaderKey = "X-Event-Key"

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
}

// WebHookInfoRepoDetails represents repository info of an incoming webhook
type WebHookInfoRepoDetails struct {
	Name  string `json:"name,omitempty"`
	Owner string `json:"owner,omitempty"`
}

// WebhookParser is a webhook parser of an incoming webhook from a VCS server
type WebhookParser interface {
	// Validate the webhook payload with the expected token and return the payload
	validatePayload(token []byte) ([]byte, error)
	// Parse the webhook payload and return WebhookInfo
	parseIncomingWebhook(payload []byte) (*WebhookInfo, error)
}

// ParseIncomingWebhook parse incoming webhook payload request into a structurized WebhookInfo object.
// provider - The VCS provider
// token    - Token to authenticate incoming webhooks. If empty, signature will not be verified.
// request  - The HTTP request of the incoming webhook
func ParseIncomingWebhook(provider vcsutils.VcsProvider, token []byte, request *http.Request) (*WebhookInfo, error) {
	if request.Body != nil {
		defer request.Body.Close()
	}
	parser := createWebhookParser(provider, request)
	payload, err := parser.validatePayload(token)
	if err != nil {
		return nil, err
	}

	webhookInfo, err := parser.parseIncomingWebhook(payload)
	if err != nil {
		return nil, err
	}
	return webhookInfo, nil
}
