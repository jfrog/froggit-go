package webhookparser

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/jfrog/froggit-go/vcsutils"
)

const gitLabKeyHeader = "X-GitLab-Token"

// GitLabWebhook represents an incoming webhook on GitLab
type GitLabWebhook struct {
	request *http.Request
}

// NewGitLabWebhook create a new GitLabWebhook instance
func NewGitLabWebhook(request *http.Request) *GitLabWebhook {
	return &GitLabWebhook{
		request: request,
	}
}

func (webhook *GitLabWebhook) validatePayload(token []byte) ([]byte, error) {
	actualToken := webhook.request.Header.Get(gitLabKeyHeader)
	if len(token) != 0 || len(actualToken) > 0 {
		if actualToken != string(token) {
			return nil, errors.New("token mismatch")
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

func (webhook *GitLabWebhook) Parse(token []byte) (*WebhookInfo, error) {
	return validateAndParseHttpRequest(webhook, token, webhook.request)
}

func (webhook *GitLabWebhook) parsePushEvent(event *gitlab.PushEvent) *WebhookInfo {
	var localTimestamp int64
	if len(event.Commits) > 0 {
		localTimestamp = event.Commits[0].Timestamp.Local().Unix()
	}
	lastCommit := optional(webhook.getLastCommit(event))
	return &WebhookInfo{
		TargetRepositoryDetails: webhook.parseRepoDetails(event.Project.PathWithNamespace),
		TargetBranch:            strings.TrimPrefix(event.Ref, "refs/heads/"),
		Timestamp:               localTimestamp,
		Event:                   vcsutils.Push,
		Commit: WebHookInfoCommit{
			Hash:    event.After,
			Message: lastCommit.Message,
			Url:     lastCommit.URL,
		},
		BeforeCommit: WebHookInfoCommit{
			Hash: event.Before,
		},
		BranchStatus: webhook.branchStatus(event),
		TriggeredBy: WebHookInfoUser{
			Login:       event.UserUsername,
			Email:       event.UserEmail,
			DisplayName: event.UserName,
			AvatarUrl:   event.UserAvatar,
		},
		Committer: WebHookInfoUser{
			DisplayName: lastCommit.Author.Name,
			Email:       lastCommit.Author.Email,
		},
		Author: WebHookInfoUser{
			DisplayName: lastCommit.Author.Name,
			Email:       lastCommit.Author.Email,
		},
	}
}

func (webhook *GitLabWebhook) getLastCommit(event *gitlab.PushEvent) *struct {
	ID        string     `json:"id"`
	Message   string     `json:"message"`
	Title     string     `json:"title"`
	Timestamp *time.Time `json:"timestamp"`
	URL       string     `json:"url"`
	Author    struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Removed  []string `json:"removed"`
} {
	if len(event.Commits) == 0 {
		return nil
	}
	return event.Commits[len(event.Commits)-1]
}

func (webhook *GitLabWebhook) parseRepoDetails(pathWithNamespace string) WebHookInfoRepoDetails {
	split := strings.Split(pathWithNamespace, "/")
	return WebHookInfoRepoDetails{
		Name:  split[1],
		Owner: split[0],
	}
}

func (webhook *GitLabWebhook) parsePrEvents(event *gitlab.MergeEvent) (*WebhookInfo, error) {
	var webhookEvent vcsutils.WebhookEvent
	switch event.ObjectAttributes.Action {
	case "open", "reopen":
		webhookEvent = vcsutils.PrOpened
	case "update":
		webhookEvent = vcsutils.PrEdited
	case "merge":
		webhookEvent = vcsutils.PrMerged
	case "close":
		webhookEvent = vcsutils.PrRejected
	default:
		//Action is not supported
		return nil, nil
	}
	eventTime, err := time.Parse("2006-01-02 15:04:05 MST", event.ObjectAttributes.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &WebhookInfo{
		PullRequestId:           event.ObjectAttributes.IID,
		SourceRepositoryDetails: webhook.parseRepoDetails(event.ObjectAttributes.Source.PathWithNamespace),
		SourceBranch:            event.ObjectAttributes.SourceBranch,
		TargetRepositoryDetails: webhook.parseRepoDetails(event.ObjectAttributes.Target.PathWithNamespace),
		TargetBranch:            event.ObjectAttributes.TargetBranch,
		Timestamp:               eventTime.UTC().Unix(),
		Event:                   webhookEvent,
	}, nil
}

func (webhook *GitLabWebhook) branchStatus(event *gitlab.PushEvent) WebHookInfoBranchStatus {
	existsAfter := event.After != gitNilHash
	existedBefore := event.Before != gitNilHash
	return branchStatus(existedBefore, existsAfter)
}
