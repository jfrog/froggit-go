package webhookparser

import (
	"net/http"

	"github.com/jfrog/froggit-go/vcsutils"
)

const EventHeaderKey = "X-Event-Key"

type WebhookInfo struct {
	Repository       string                `json:"repository,omitempty"`        // The target repository on pull requests and push
	Branch           string                `json:"branch,omitempty"`            // The target branch on pull requests and push
	SourceRepository string                `json:"source_repository,omitempty"` // The source repository on pull requests
	SourceBranch     string                `json:"source_branch,omitempty"`     // The source branch on pull requests
	Timestamp        int64                 `json:"timestamp,omitempty"`         // Seconds from epoch
	Event            vcsutils.WebhookEvent `json:"event,omitempty"`             // The event type
}

type WebhookParser interface {
	validatePayload(token []byte) ([]byte, error)
	parseIncomingWebhook(token []byte) (*WebhookInfo, error)
}

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
