package webhookparser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v45/github"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

// gitHubWebhookParser represents an incoming webhook on GitHub
type gitHubWebhookParser struct {
	logger vcsclient.Log
	// Used for GitHub On-prem
	endpoint string
}

// newGitHubWebhookParser create a new gitHubWebhookParser instance
func newGitHubWebhookParser(logger vcsclient.Log, endpoint string) *gitHubWebhookParser {
	if endpoint == "" {
		// Default to GitHub "Cloud"
		endpoint = "https://github.com"
	} else {
		// For GitHub the API endpoint is https://api.github.com but the Web Interface URL is https://github.com
		// So we remove the "api." prefix to the hostname
		// Applied to Cloud and On-Prem versions of GitHub
		endpoint = strings.Replace(endpoint, "://api.", "://", 1)
	}
	logger.Debug("Github URL: ", endpoint)
	return &gitHubWebhookParser{
		logger:   logger,
		endpoint: endpoint,
	}
}

func (webhook *gitHubWebhookParser) validatePayload(_ context.Context, request *http.Request, token []byte) ([]byte, error) {
	// Make sure X-Hub-Signature-256 header exist
	if len(token) > 0 && len(request.Header.Get(github.SHA256SignatureHeader)) == 0 {
		return nil, errors.New(github.SHA256SignatureHeader + " header is missing")
	}

	payload, err := github.ValidatePayload(request, token)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (webhook *gitHubWebhookParser) parseIncomingWebhook(_ context.Context, request *http.Request, payload []byte) (*WebhookInfo, error) {
	event, err := github.ParseWebHook(github.WebHookType(request), payload)
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

func (webhook *gitHubWebhookParser) parsePushEvent(event *github.PushEvent) *WebhookInfo {
	repoDetails := WebHookInfoRepoDetails{
		Name:  optional(optional(event.GetRepo()).Name),
		Owner: optional(optional(optional(event.GetRepo()).Owner).Login),
	}
	compareURL := ""
	if webhook.endpoint != "" {
		compareURL = fmt.Sprintf("%s/%s/%s/compare/%s...%s", webhook.endpoint, repoDetails.Owner, repoDetails.Name,
			event.GetBefore(), event.GetAfter())
	}
	return &WebhookInfo{
		TargetRepositoryDetails: repoDetails,
		TargetBranch:            webhook.trimRefPrefix(event.GetRef()),
		Timestamp:               event.GetHeadCommit().GetTimestamp().UTC().Unix(),
		Event:                   vcsutils.Push,
		Commit: WebHookInfoCommit{
			Hash:    event.GetAfter(),
			Message: optional(optional(event.HeadCommit).Message),
			Url:     optional(optional(event.HeadCommit).URL),
		},
		BeforeCommit: WebHookInfoCommit{
			Hash: event.GetBefore(),
		},
		BranchStatus: webhook.branchStatus(event),
		TriggeredBy:  webhook.user(event.Pusher),
		Committer:    webhook.commitAuthor(optional(event.HeadCommit).Committer),
		Author:       webhook.commitAuthor(optional(event.HeadCommit).Author),
		CompareUrl:   compareURL,
	}
}

func (webhook *gitHubWebhookParser) trimRefPrefix(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func (webhook *gitHubWebhookParser) user(u *github.User) WebHookInfoUser {
	if u == nil {
		return WebHookInfoUser{}
	}
	return WebHookInfoUser{
		Login:       optional(u.Login),
		DisplayName: optional(u.Name),
		Email:       optional(u.Email),
		AvatarUrl:   "",
	}
}

func (webhook *gitHubWebhookParser) commitAuthor(u *github.CommitAuthor) WebHookInfoUser {
	if u == nil {
		return WebHookInfoUser{}
	}
	return WebHookInfoUser{
		Login:       optional(u.Login),
		DisplayName: optional(u.Name),
		Email:       optional(u.Email),
		AvatarUrl:   "",
	}
}

func (webhook *gitHubWebhookParser) parsePrEvents(event *github.PullRequestEvent) *WebhookInfo {
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

func (webhook *gitHubWebhookParser) resolveClosedEventType(event *github.PullRequestEvent) vcsutils.WebhookEvent {
	if event.GetPullRequest().GetMerged() {
		return vcsutils.PrMerged
	}
	return vcsutils.PrRejected
}

func (webhook *gitHubWebhookParser) branchStatus(event *github.PushEvent) WebHookInfoBranchStatus {
	existsAfter := event.GetAfter() != gitNilHash
	existedBefore := event.GetBefore() != gitNilHash
	return branchStatus(existedBefore, existsAfter)
}
