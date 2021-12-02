package webhookparser

import (
	"net/http"

	"github.com/jfrog/froggit-go/vcsutils"
)

const EventHeaderKey = "X-Event-Key"

// This struct is used for parsing an incoming webhook request from the VCS provider.
type WebhookInfo struct {
	// The target repository for pull requests and push
	Repository string `json:"repository,omitempty"`
	// The target branch for pull requests and push
	Branch string `json:"branch,omitempty"`
	// Pull request id
	PullRequestId int `json:"pull_request_id,omitempty"`
	// The source repository for pull requests
	SourceRepository string `json:"source_repository,omitempty"`
	// The source branch for pull requests
	SourceBranch string `json:"source_branch,omitempty"`
	// Seconds from epoch
	Timestamp int64 `json:"timestamp,omitempty"`
	// The event type
	Event vcsutils.WebhookEvent `json:"event,omitempty"`
}

type WebhookParser interface {
	// Validate the webhook payload with the expected token and return the payload
	validatePayload(token []byte) ([]byte, error)
	// Parse the webhook payload and return WebhookInfo
	parseIncomingWebhook(payload []byte) (*WebhookInfo, error)
}

// Parse incoming webhook payload request into a structurized WebhookInfo object.
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
