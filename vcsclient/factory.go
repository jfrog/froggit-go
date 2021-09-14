package vcsclient

import (
	"context"
	"log"

	"github.com/jfrog/froggit-go/vcsutils"
)

type ClientBuilder struct {
	vcsProvider vcsutils.VcsProvider
	vcsInfo     VcsInfo
	context     context.Context
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

func (builder *ClientBuilder) Context(context context.Context) *ClientBuilder {
	builder.context = context
	return builder
}

func (builder *ClientBuilder) Logger(logger *log.Logger) *ClientBuilder {
	builder.logger = logger
	return builder
}

func (builder *ClientBuilder) Build() (VcsClient, error) {
	ctx := builder.context
	if ctx == nil {
		ctx = context.Background()
	}
	switch builder.vcsProvider {
	case vcsutils.GitHub:
		return NewGitHubClient(ctx, &builder.vcsInfo)
	case vcsutils.GitLab:
		return NewGitLabClient(ctx, &builder.vcsInfo)
	case vcsutils.BitbucketServer:
		return NewBitbucketServerClient(ctx, &builder.vcsInfo)
	case vcsutils.BitbucketCloud:
		return NewBitbucketCloudClient(ctx, &builder.vcsInfo)
	}
	return nil, nil
}
