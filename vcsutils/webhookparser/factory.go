package webhookparser

import (
	"strings"

	"github.com/jfrog/froggit-go/vcsutils"
)

func createWebhookParser(logger vcsutils.Log, origin WebhookOrigin) webhookParser {
	origin.OriginURL = strings.TrimSuffix(origin.OriginURL, "/")
	switch origin.VcsProvider {
	case vcsutils.GitHub:
		return newGitHubWebhookParser(logger, origin.OriginURL)
	case vcsutils.GitLab:
		return newGitLabWebhookParser(logger)
	case vcsutils.BitbucketServer:
		return newBitbucketServerWebhookParser(logger, origin.OriginURL)
	case vcsutils.BitbucketCloud:
		return newBitbucketCloudWebhookParser(logger)
	}
	return nil
}
