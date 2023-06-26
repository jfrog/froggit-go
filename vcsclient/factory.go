package vcsclient

import (
	"github.com/jfrog/froggit-go/vcsutils"
)

// ClientBuilder builds VcsClient
type ClientBuilder struct {
	vcsProvider vcsutils.VcsProvider
	vcsInfo     vcsutils.VcsInfo
	logger      Log
}

// NewClientBuilder creates new ClientBuilder
func NewClientBuilder(vcsProvider vcsutils.VcsProvider) *ClientBuilder {
	return &ClientBuilder{vcsProvider: vcsProvider, logger: EmptyLogger{}}
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
func (builder *ClientBuilder) Logger(logger Log) *ClientBuilder {
	builder.logger = logger
	return builder
}

// Project sets the project
func (builder *ClientBuilder) Project(project string) *ClientBuilder {
	builder.vcsInfo.Project = project
	return builder
}

// Build builds the VcsClient
func (builder *ClientBuilder) Build() (VcsClient, error) {
	switch builder.vcsProvider {
	case vcsutils.GitHub:
		return NewGitHubClient(builder.vcsInfo, builder.logger)
	case vcsutils.GitLab:
		return NewGitLabClient(builder.vcsInfo, builder.logger)
	case vcsutils.BitbucketServer:
		return NewBitbucketServerClient(builder.vcsInfo, builder.logger)
	case vcsutils.BitbucketCloud:
		return NewBitbucketCloudClient(builder.vcsInfo, builder.logger)
	case vcsutils.AzureRepos:
		return NewAzureReposClient(builder.vcsInfo, builder.logger)
	}
	return nil, nil
}
