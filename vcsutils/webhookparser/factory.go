package webhookparser

import (
	"net/http"

	"github.com/jfrog/froggit-go/vcsutils"
)

// ParserBuilder builds VcsClient
type ParserBuilder struct {
	vcsProvider vcsutils.VcsProvider
	request     *http.Request
	endpoint    string
}

// NewParserBuilder creates new ParserBuilder
func NewParserBuilder(vcsProvider vcsutils.VcsProvider, request *http.Request) *ParserBuilder {
	return &ParserBuilder{
		request:     request,
		endpoint:    "undefined",
		vcsProvider: vcsProvider,
	}
}

// ApiEndpoint sets the API endpoint
func (builder *ParserBuilder) ApiEndpoint(apiEndpoint string) *ParserBuilder {
	builder.endpoint = apiEndpoint
	return builder
}

// Build builds the VcsClient
func (builder *ParserBuilder) Build() WebhookParser {
	switch builder.vcsProvider {
	case vcsutils.GitHub:
		return NewGitHubWebhook(builder.request)
	case vcsutils.GitLab:
		return NewGitLabWebhook(builder.request)
	case vcsutils.BitbucketServer:
		return NewBitbucketServerWebhookWebhook(builder.request,
			WithBitbucketServerEndpoint(builder.endpoint))
	case vcsutils.BitbucketCloud:
		return NewBitbucketCloudWebhookWebhook(builder.request)
	}
	return nil
}
