package webhookparser

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/go-github/v38/github"
	"github.com/jfrog/froggit-go/vcsutils"
)

type GitHubWebhook struct {
	request *http.Request
}

func NewGitHubWebhook(request *http.Request) *GitHubWebhook {
	return &GitHubWebhook{
		request: request,
	}
}

func (webhook *GitHubWebhook) validatePayload(token []byte) ([]byte, error) {
	// Make sure X-Hub-Signature-256 header exist
	if len(token) > 0 && len(webhook.request.Header.Get(github.SHA256SignatureHeader)) == 0 {
		return nil, errors.New(github.SHA256SignatureHeader + " header is missing.")
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
		Repository: event.GetRepo().GetFullName(),
		Branch:     strings.TrimPrefix(event.GetRef(), "refs/heads/"),
		Timestamp:  event.GetHeadCommit().GetTimestamp().UTC().Unix(),
		Event:      vcsutils.Push,
	}
}

func (webhook *GitHubWebhook) parsePrEvents(event *github.PullRequestEvent) *WebhookInfo {
	var webhookEvent vcsutils.WebhookEvent
	if event.GetAction() == "opened" {
		webhookEvent = vcsutils.PrCreated
	} else if event.GetAction() == "synchronize" {
		webhookEvent = vcsutils.PrEdited
	} else {
		// Action is not supported
		return nil
	}
	return &WebhookInfo{
		Repository:       event.GetPullRequest().GetBase().GetRepo().GetFullName(),
		Branch:           event.GetPullRequest().GetBase().GetRef(),
		SourceRepository: event.GetPullRequest().GetHead().GetRepo().GetFullName(),
		SourceBranch:     event.GetPullRequest().GetHead().GetRef(),
		Timestamp:        event.GetPullRequest().GetUpdatedAt().UTC().Unix(),
		Event:            webhookEvent,
	}
}
