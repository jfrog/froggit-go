package vcsutils

type VcsProvider int

const (
	GitHub VcsProvider = iota
	GitLab
	BitbucketServer
	BitbucketCloud
)

func (v VcsProvider) String() string {
	switch v {
	case GitHub:
		return "GitHub"
	case GitLab:
		return "GitLab"
	case BitbucketServer:
		return "Bitbucket Server"
	case BitbucketCloud:
		return "Bitbucket Cloud"
	default:
		return ""
	}
}

type WebhookEvent string

const (
	PrRejected WebhookEvent = "PrRejected"
	PrEdited   WebhookEvent = "PrEdited"
	PrMerged   WebhookEvent = "PrMerged"
	PrOpened   WebhookEvent = "PrOpened"
	Push       WebhookEvent = "Push"
)
