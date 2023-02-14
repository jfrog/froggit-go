package webhookparser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

func TestCreateWebhookParser(t *testing.T) {
	assert.IsType(t, &gitHubWebhookParser{}, newParser(vcsutils.GitHub))
	assert.IsType(t, &gitLabWebhookParser{}, newParser(vcsutils.GitLab))
	assert.IsType(t, &bitbucketServerWebhookParser{}, newParser(vcsutils.BitbucketServer))
	assert.IsType(t, &bitbucketCloudWebhookParser{}, newParser(vcsutils.BitbucketCloud))
	assert.Nil(t, newParser(5))
}

func newParser(provider vcsutils.VcsProvider) webhookParser {
	return createWebhookParser(
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: provider,
		})
}
