package webhookparser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v56/github"

	"github.com/jfrog/froggit-go/vcsutils"
)

// gitHubWebhookParser represents an incoming webhook on GitHub
type gitHubWebhookParser struct {
	logger vcsutils.Log
	// Used for GitHub On-prem
	endpoint string
}

// newGitHubWebhookParser create a new gitHubWebhookParser instance
func newGitHubWebhookParser(logger vcsutils.Log, endpoint string) *gitHubWebhookParser {
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
		if webhook.isTagEvent(event) {
			return webhook.parseTagEvent(event), nil
		}
		return webhook.parseChangeEvent(event), nil
	case *github.PullRequestEvent:
		return webhook.parsePrEvents(event), nil
	}
	return nil, nil
}

func (webhook *gitHubWebhookParser) parseTagEvent(event *github.PushEvent) *WebhookInfo {
	info := &WebhookInfo{
		Tag: &WebhookInfoTag{
			Name: strings.TrimPrefix(event.GetRef(), "refs/tags/"),
			Repository: WebHookInfoRepoDetails{
				Name:  vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(event.GetRepo()).Name),
				Owner: vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(event.GetRepo()).Owner).Login),
			},
			Author: webhook.user(event.Pusher),
		},
	}
	if event.GetCreated() {
		info.Event = vcsutils.TagPushed
		info.Tag.Hash = event.GetAfter()
	} else if event.GetDeleted() {
		info.Event = vcsutils.TagRemoved
		info.Tag.Hash = event.GetBefore()
	}
	return info
}

func (webhook *gitHubWebhookParser) parseChangeEvent(event *github.PushEvent) *WebhookInfo {
	repoDetails := WebHookInfoRepoDetails{
		Name:  vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(event.GetRepo()).Name),
		Owner: vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(event.GetRepo()).Owner).Login),
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
			Message: vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(event.HeadCommit).Message),
			Url:     vcsutils.DefaultIfNotNil(vcsutils.DefaultIfNotNil(event.HeadCommit).URL),
		},
		BeforeCommit: WebHookInfoCommit{
			Hash: event.GetBefore(),
		},
		BranchStatus: webhook.branchStatus(event),
		TriggeredBy:  webhook.user(event.Pusher),
		Committer:    webhook.commitAuthor(vcsutils.DefaultIfNotNil(event.HeadCommit).Committer),
		Author:       webhook.commitAuthor(vcsutils.DefaultIfNotNil(event.HeadCommit).Author),
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
		Login:       vcsutils.DefaultIfNotNil(u.Login),
		DisplayName: vcsutils.DefaultIfNotNil(u.Name),
		Email:       vcsutils.DefaultIfNotNil(u.Email),
		AvatarUrl:   "",
	}
}

func (webhook *gitHubWebhookParser) commitAuthor(u *github.CommitAuthor) WebHookInfoUser {
	if u == nil {
		return WebHookInfoUser{}
	}
	return WebHookInfoUser{
		Login:       vcsutils.DefaultIfNotNil(u.Login),
		DisplayName: vcsutils.DefaultIfNotNil(u.Name),
		Email:       vcsutils.DefaultIfNotNil(u.Email),
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
	pullRequest := event.GetPullRequest()
	targetRepository := WebHookInfoRepoDetails{
		Name:  pullRequest.GetBase().GetRepo().GetName(),
		Owner: pullRequest.GetBase().GetRepo().GetOwner().GetLogin(),
	}
	sourceRepository := WebHookInfoRepoDetails{
		Name:  pullRequest.GetHead().GetRepo().GetName(),
		Owner: pullRequest.GetHead().GetRepo().GetOwner().GetLogin(),
	}
	return &WebhookInfo{
		PullRequestId:           pullRequest.GetNumber(),
		TargetRepositoryDetails: targetRepository,
		TargetBranch:            pullRequest.GetBase().GetRef(),
		SourceRepositoryDetails: sourceRepository,
		SourceBranch:            pullRequest.GetHead().GetRef(),
		Timestamp:               pullRequest.GetUpdatedAt().UTC().Unix(),
		Event:                   webhookEvent,

		PullRequest: &WebhookInfoPullRequest{
			ID:         pullRequest.GetNumber(),
			Title:      pullRequest.GetTitle(),
			CompareUrl: pullRequest.GetHTMLURL() + "/files",
			Timestamp:  pullRequest.GetUpdatedAt().Unix(),
			Author: WebHookInfoUser{
				Login:       pullRequest.GetUser().GetLogin(),
				DisplayName: pullRequest.GetUser().GetName(),
				Email:       pullRequest.GetUser().GetEmail(),
				AvatarUrl:   pullRequest.GetUser().GetAvatarURL(),
			},
			TriggeredBy: WebHookInfoUser{
				Login:     event.GetSender().GetLogin(),
				AvatarUrl: event.GetSender().GetAvatarURL(),
			},
			TargetRepository: targetRepository,
			TargetBranch:     pullRequest.GetBase().GetRef(),
			TargetHash:       pullRequest.GetBase().GetSHA(),
			SourceRepository: sourceRepository,
			SourceBranch:     pullRequest.GetHead().GetRef(),
			SourceHash:       pullRequest.GetHead().GetSHA(),
		},
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

func (webhook *gitHubWebhookParser) isTagEvent(event *github.PushEvent) bool {
	return strings.HasPrefix(event.GetRef(), "refs/tags/")
}
