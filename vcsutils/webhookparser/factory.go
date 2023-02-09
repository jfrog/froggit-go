package webhookparser

import (
	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

func createWebhookParser(logger vcsclient.Log, origin WebhookOrigin) WebhookParser {
	switch origin.VcsProvider {
	case vcsutils.GitHub:
		return NewGitHubWebhook(logger, origin.OriginURL)
	case vcsutils.GitLab:
		return NewGitLabWebhook(logger)
	case vcsutils.BitbucketServer:
		return NewBitbucketServerWebhookWebhook(logger, origin.OriginURL)
	case vcsutils.BitbucketCloud:
		return NewBitbucketCloudWebhookWebhook(logger)
	}
	return nil
}
