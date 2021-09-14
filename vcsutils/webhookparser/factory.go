package webhookparser

import (
	"net/http"

	"github.com/jfrog/froggit-go/vcsutils"
)

func createWebhookParser(provider vcsutils.VcsProvider, request *http.Request) WebhookParser {
	switch provider {
	case vcsutils.GitHub:
		return NewGitHubWebhook(request)
	case vcsutils.BitbucketCloud:
		return NewBitbucketCloudWebhookWebhook(request)
	case vcsutils.BitbucketServer:
		return NewBitbucketServerWebhookWebhook(request)
	case vcsutils.GitLab:
		return NewGitLabWebhook(request)
	}
	return nil
}
