package webhookparser

import (
	"testing"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

func TestCreateWebhookParser(t *testing.T) {
	assert.IsType(t, &GitHubWebhook{}, createWebhookParser(vcsutils.GitHub, nil))
	assert.IsType(t, &GitLabWebhook{}, createWebhookParser(vcsutils.GitLab, nil))
	assert.IsType(t, &BitbucketServerWebhook{}, createWebhookParser(vcsutils.BitbucketServer, nil))
	assert.IsType(t, &BitbucketCloudWebhook{}, createWebhookParser(vcsutils.BitbucketCloud, nil))
	assert.Nil(t, createWebhookParser(5, nil))
}
