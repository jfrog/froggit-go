package webhookparser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

func TestCreateWebhookParser(t *testing.T) {
	assert.IsType(t, &GitHubWebhook{}, newParser(vcsutils.GitHub))
	assert.IsType(t, &GitLabWebhook{}, newParser(vcsutils.GitLab))
	assert.IsType(t, &BitbucketServerWebhook{}, newParser(vcsutils.BitbucketServer))
	assert.IsType(t, &BitbucketCloudWebhook{}, newParser(vcsutils.BitbucketCloud))
	assert.Nil(t, newParser(5))
}

func newParser(provider vcsutils.VcsProvider) WebhookParser {
	return createWebhookParser(
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: provider,
		})
}
