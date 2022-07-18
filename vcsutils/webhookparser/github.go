package webhookparser

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/jfrog/froggit-go/vcsutils"
)

// GitHubWebhook represents an incoming webhook on GitHub
type GitHubWebhook struct {
	request *http.Request
}

// NewGitHubWebhook create a new GitHubWebhook instance
func NewGitHubWebhook(request *http.Request) *GitHubWebhook {
	return &GitHubWebhook{
		request: request,
	}
}

func (webhook *GitHubWebhook) validatePayload(token []byte) ([]byte, error) {
	// Make sure X-Hub-Signature-256 header exist
	if len(token) > 0 && len(webhook.request.Header.Get(github.SHA256SignatureHeader)) == 0 {
		return nil, errors.New(github.SHA256SignatureHeader + " header is missing")
	}

	payload, err := github.ValidatePayload(webhook.request, token)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (webhook *GitHubWebhook) parseIncomingWebhook(payload []byte) (*WebhookInfo, error) {
	event, err := github.ParseWebHook(github.WebHookType(webhook.request), payload)
	if err != nil {
		return nil, err
	}
	switch event := event.(type) {
	case *github.PushEvent:
		return webhook.parsePushEvent(event), nil
	case *github.PullRequestEvent:
		return webhook.parsePrEvents(event), nil
	}
	return nil, nil
}

func (webhook *GitHubWebhook) parsePushEvent(event *github.PushEvent) *WebhookInfo {
	return &WebhookInfo{
		TargetRepositoryDetails: WebHookInfoRepoDetails{
			Name:  *event.GetRepo().Name,
			Owner: *event.GetRepo().Owner.Login,
		},
		TargetBranch: strings.TrimPrefix(event.GetRef(), "refs/heads/"),
		Timestamp:    event.GetHeadCommit().GetTimestamp().UTC().Unix(),
		Event:        vcsutils.Push,
	}
}

func (webhook *GitHubWebhook) parsePrEvents(event *github.PullRequestEvent) *WebhookInfo {
	var webhookEvent vcsutils.WebhookEvent
	switch event.GetAction() {
	case "opened", "reopened":
		webhookEvent = vcsutils.PrOpened
	case "synchronize", "edited":
		webhookEvent = vcsutils.PrEdited
	case "closed":
		webhookEvent = webhook.resolveClosedEventType(event)
	default:
		// Action is not supported
		return nil
	}
	return &WebhookInfo{
		PullRequestId: event.GetPullRequest().GetNumber(),
		TargetRepositoryDetails: WebHookInfoRepoDetails{
			Name:  *event.GetPullRequest().GetBase().GetRepo().Name,
			Owner: *event.GetPullRequest().GetBase().GetRepo().Owner.Login,
		},
		TargetBranch: event.GetPullRequest().GetBase().GetRef(),
		SourceRepositoryDetails: WebHookInfoRepoDetails{
			Name:  *event.GetPullRequest().GetHead().GetRepo().Name,
			Owner: *event.GetPullRequest().GetHead().GetRepo().Owner.Login,
		},
		SourceBranch: event.GetPullRequest().GetHead().GetRef(),
		Timestamp:    event.GetPullRequest().GetUpdatedAt().UTC().Unix(),
		Event:        webhookEvent,
	}
}

func (webhook *GitHubWebhook) resolveClosedEventType(event *github.PullRequestEvent) vcsutils.WebhookEvent {
	if event.GetPullRequest().GetMerged() {
		return vcsutils.PrMerged
	}
	return vcsutils.PrRejected
}
