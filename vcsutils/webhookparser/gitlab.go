package webhookparser

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/xanzy/go-gitlab"
)

const gitLabKeyHeader = "X-GitLab-Token"

type GitLabWebhook struct {
	request *http.Request
}

func NewGitLabWebhook(request *http.Request) *GitLabWebhook {
	return &GitLabWebhook{
		request: request,
	}
}

func (webhook *GitLabWebhook) validatePayload(token []byte) ([]byte, error) {
	actualToken := webhook.request.Header.Get(gitLabKeyHeader)
	if len(token) != 0 || len(actualToken) > 0 {
		if actualToken != string(token) {
			return nil, errors.New("Token mismatch")
		}
	}

	payload := new(bytes.Buffer)
	if _, err := payload.ReadFrom(webhook.request.Body); err != nil {
		return nil, err
	}
	return payload.Bytes(), nil
}
func (webhook *GitLabWebhook) parseIncomingWebhook(payload []byte) (*WebhookInfo, error) {
	event, err := gitlab.ParseWebhook(gitlab.WebhookEventType(webhook.request), payload)
	if err != nil {
		return nil, err
	}
	switch event := event.(type) {
	case *gitlab.PushEvent:
		return webhook.parsePushEvent(event), nil
	case *gitlab.MergeEvent:
		return webhook.parsePrEvents(event)
	}
	return nil, nil
}

func (webhook *GitLabWebhook) parsePushEvent(event *gitlab.PushEvent) *WebhookInfo {
	return &WebhookInfo{
		Repository: event.Project.PathWithNamespace,
		Branch:     strings.TrimPrefix(event.Ref, "refs/heads/"),
		Timestamp:  event.Commits[0].Timestamp.Local().Unix(),
		Event:      vcsutils.Push,
	}
}

func (webhook *GitLabWebhook) parsePrEvents(event *gitlab.MergeEvent) (*WebhookInfo, error) {
	var webhookEvent vcsutils.WebhookEvent
	if event.ObjectAttributes.CreatedAt == event.ObjectAttributes.UpdatedAt {
		webhookEvent = vcsutils.PrCreated
	} else {
		webhookEvent = vcsutils.PrEdited
	}
	time, err := time.Parse("2006-01-02 15:04:05 MST", event.ObjectAttributes.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &WebhookInfo{
		SourceRepository: event.ObjectAttributes.Source.PathWithNamespace,
		SourceBranch:     event.ObjectAttributes.SourceBranch,
		Repository:       event.ObjectAttributes.Target.PathWithNamespace,
		Branch:           event.ObjectAttributes.TargetBranch,
		Timestamp:        time.UTC().Unix(),
		Event:            webhookEvent,
	}, nil
}
