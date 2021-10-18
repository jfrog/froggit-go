package vcsclient

import (
	"log"

	"github.com/jfrog/froggit-go/vcsutils"
)

type ClientBuilder struct {
	vcsProvider vcsutils.VcsProvider
	vcsInfo     VcsInfo
	logger      *log.Logger
}

func NewClientBuilder(vcsProvider vcsutils.VcsProvider) *ClientBuilder {
	return &ClientBuilder{vcsProvider: vcsProvider}
}

func (builder *ClientBuilder) ApiEndpoint(apiEndpoint string) *ClientBuilder {
	builder.vcsInfo.ApiEndpoint = apiEndpoint
	return builder
}

func (builder *ClientBuilder) Username(username string) *ClientBuilder {
	builder.vcsInfo.Username = username
	return builder
}

func (builder *ClientBuilder) Token(token string) *ClientBuilder {
	builder.vcsInfo.Token = token
	return builder
}

func (builder *ClientBuilder) Logger(logger *log.Logger) *ClientBuilder {
	builder.logger = logger
	return builder
}

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
