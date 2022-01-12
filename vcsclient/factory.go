package vcsclient

import (
	"log"

	"github.com/jfrog/froggit-go/vcsutils"
)

// ClientBuilder builds VcsClient
type ClientBuilder struct {
	vcsProvider vcsutils.VcsProvider
	vcsInfo     VcsInfo
	logger      *log.Logger
}

// NewClientBuilder creates new ClientBuilder
func NewClientBuilder(vcsProvider vcsutils.VcsProvider) *ClientBuilder {
	return &ClientBuilder{vcsProvider: vcsProvider}
}

// ApiEndpoint sets the API endpoint
func (builder *ClientBuilder) ApiEndpoint(apiEndpoint string) *ClientBuilder {
	builder.vcsInfo.APIEndpoint = apiEndpoint
	return builder
}

// Username sets the username
func (builder *ClientBuilder) Username(username string) *ClientBuilder {
	builder.vcsInfo.Username = username
	return builder
}

// Token sets the access token
func (builder *ClientBuilder) Token(token string) *ClientBuilder {
	builder.vcsInfo.Token = token
	return builder
}

// Logger sets the logger
func (builder *ClientBuilder) Logger(logger *log.Logger) *ClientBuilder {
	builder.logger = logger
	return builder
}

// Build builds the VcsClient
func (builder *ClientBuilder) Build() (VcsClient, error) {
	switch builder.vcsProvider {
	case vcsutils.GitHub:
		return NewGitHubClient(builder.vcsInfo)
	case vcsutils.GitLab:
		return NewGitLabClient(builder.vcsInfo)
	case vcsutils.BitbucketServer:
		return NewBitbucketServerClient(builder.vcsInfo)
	case vcsutils.BitbucketCloud:
		return NewBitbucketCloudClient(builder.vcsInfo)
	}
	return nil, nil
}
