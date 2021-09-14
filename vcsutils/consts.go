package vcsutils

type VcsProvider int

const (
	GitHub VcsProvider = iota
	GitLab
	BitbucketServer
	BitbucketCloud
)

type WebhookEvent string

const (
	PrCreated WebhookEvent = "PrCreated"
	PrEdited  WebhookEvent = "PrEdited"
	Push      WebhookEvent = "Push"
)
