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
	PrCreated WebhookEvent = "PrCreated"
	PrEdited  WebhookEvent = "PrEdited"
	Push      WebhookEvent = "Push"
)
