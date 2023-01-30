package webhookparser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jfrog/froggit-go/vcsutils"
)

func TestCreateWebhookParser(t *testing.T) {
	assert.IsType(t, &GitHubWebhook{}, createWebhookParser(vcsutils.GitHub))
	assert.IsType(t, &GitLabWebhook{}, createWebhookParser(vcsutils.GitLab))
	assert.IsType(t, &BitbucketServerWebhook{}, createWebhookParser(vcsutils.BitbucketServer))
	assert.IsType(t, &BitbucketCloudWebhook{}, createWebhookParser(vcsutils.BitbucketCloud))
	assert.Nil(t, createWebhookParser(5))
}

func createWebhookParser(provider vcsutils.VcsProvider) WebhookParser {
	return NewParserBuilder(provider, nil).Build()
}
