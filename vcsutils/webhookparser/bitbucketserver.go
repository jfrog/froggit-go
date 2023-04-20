package webhookparser

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

const sha256Signature = "X-Hub-Signature"
const bitbucketServerEventHeader = "X-Event-Key"

// bitbucketServerWebhookParser represents an incoming webhook on Bitbucket server
type bitbucketServerWebhookParser struct {
	logger   vcsclient.Log
	endpoint string
}

// newBitbucketServerWebhookParser create a new bitbucketServerWebhookParser instance
func newBitbucketServerWebhookParser(logger vcsclient.Log, endpoint string) *bitbucketServerWebhookParser {
	return &bitbucketServerWebhookParser{
		logger:   logger,
		endpoint: endpoint,
	}
}

func (webhook *bitbucketServerWebhookParser) validatePayload(_ context.Context, request *http.Request, token []byte) ([]byte, error) {
	payload := new(bytes.Buffer)
	if _, err := payload.ReadFrom(request.Body); err != nil {
		return nil, err
	}

	expectedSignature := request.Header.Get(sha256Signature)
	if len(token) > 0 || len(expectedSignature) > 0 {
		actualSignature := calculatePayloadSignature(payload.Bytes(), token)
		if expectedSignature != "sha256="+actualSignature {
			return nil, errors.New("payload signature mismatch")
		}
	}
	return payload.Bytes(), nil
}

func (webhook *bitbucketServerWebhookParser) parseIncomingWebhook(_ context.Context, request *http.Request, payload []byte) (*WebhookInfo, error) {
	bitbucketServerWebHook := &bitbucketServerWebHook{}
	err := json.Unmarshal(payload, bitbucketServerWebHook)
	if err != nil {
		return nil, err
	}

	event := request.Header.Get(bitbucketServerEventHeader)
	switch event {
	case "repo:refs_changed":
		return webhook.parseRefChange(bitbucketServerWebHook)
	case "pr:opened":
		return webhook.parsePrEvents(bitbucketServerWebHook, vcsutils.PrOpened)
	case "pr:from_ref_updated":
		return webhook.parsePrEvents(bitbucketServerWebHook, vcsutils.PrEdited)
	case "pr:merged":
		return webhook.parsePrEvents(bitbucketServerWebHook, vcsutils.PrMerged)
	case "pr:declined", "pr:deleted":
		return webhook.parsePrEvents(bitbucketServerWebHook, vcsutils.PrRejected)
	}
	return nil, nil
}

func calculatePayloadSignature(payload []byte, token []byte) string {
	hmacHash := hmac.New(sha256.New, token)
	hmacHash.Write(payload)
	return hex.EncodeToString(hmacHash.Sum(nil))
}

func (webhook *bitbucketServerWebhookParser) parsePushEvent(bitbucketCloudWebHook *bitbucketServerWebHook) (*WebhookInfo, error) {
	eventTime, err := time.Parse("2006-01-02T15:04:05-0700", bitbucketCloudWebHook.Date)
	if err != nil {
		return nil, err
	}
	repository := bitbucketCloudWebHook.Repository
	repositoryDetails := webhook.getRepositoryDetails(repository)
	commitURL := ""
	if webhook.endpoint != "" {
		commitURL = fmt.Sprintf("%s/projects/%s/repos/%s/commits/%s", webhook.endpoint,
			repositoryDetails.Owner, repositoryDetails.Name, bitbucketCloudWebHook.Changes[0].ToHash)
	}
	return &WebhookInfo{
		TargetRepositoryDetails: repositoryDetails,
		TargetBranch:            strings.TrimPrefix(bitbucketCloudWebHook.Changes[0].RefID, "refs/heads/"),
		Timestamp:               eventTime.UTC().Unix(),
		Event:                   vcsutils.Push,
		Commit: WebHookInfoCommit{
			Hash: bitbucketCloudWebHook.Changes[0].ToHash,
			Url:  commitURL,
		},
		BeforeCommit: WebHookInfoCommit{
			Hash: bitbucketCloudWebHook.Changes[0].FromHash,
		},
		BranchStatus: webhook.branchStatus(bitbucketCloudWebHook.Changes[0].ToHash, bitbucketCloudWebHook.Changes[0].FromHash),
		TriggeredBy: WebHookInfoUser{
			Login:       bitbucketCloudWebHook.Actor.Name,
			DisplayName: bitbucketCloudWebHook.Actor.DisplayName,
		},
		Committer: WebHookInfoUser{
			Email:       bitbucketCloudWebHook.Actor.EmailAddress,
			DisplayName: bitbucketCloudWebHook.Actor.DisplayName,
		},
		Author: WebHookInfoUser{
			Email:       bitbucketCloudWebHook.Actor.EmailAddress,
			DisplayName: bitbucketCloudWebHook.Actor.DisplayName,
		},
	}, nil
}

func (webhook *bitbucketServerWebhookParser) parseRefChange(bitbucketCloudWebHook *bitbucketServerWebHook) (*WebhookInfo, error) {
	for _, change := range bitbucketCloudWebHook.Changes {
		if change.Ref.Type == "BRANCH" {
			return webhook.parsePushEvent(bitbucketCloudWebHook)
		}

		if change.Ref.Type == "TAG" {
			return webhook.parseTagEvent(bitbucketCloudWebHook, change), nil
		}
	}
	return nil, nil
}

func (webhook *bitbucketServerWebhookParser) parseTagEvent(hook *bitbucketServerWebHook, change bitbucketServerWebHookChanges) *WebhookInfo {
	webhookInfo := &WebhookInfo{
		Tag: &WebhookInfoTag{
			Name:       change.Ref.DisplayId,
			Repository: webhook.getRepositoryDetails(hook.Repository),
			Author: WebHookInfoUser{
				Login:       hook.Actor.Name,
				DisplayName: hook.Actor.DisplayName,
				AvatarUrl:   webhook.getFirstHrefFromLinks(hook.Actor.Links),
				Email:       hook.Actor.EmailAddress,
			},
		},
	}

	switch change.Type {
	case "ADD":
		webhookInfo.Event = vcsutils.TagPushed
		webhookInfo.Tag.Hash = change.ToHash
	case "DELETE":
		webhookInfo.Event = vcsutils.TagRemoved
		webhookInfo.Tag.Hash = change.FromHash
	default:
		return nil
	}

	return webhookInfo
}

func (webhook *bitbucketServerWebhookParser) getRepositoryDetails(repository bitbucketv1.Repository) WebHookInfoRepoDetails {
	return WebHookInfoRepoDetails{
		Name:  repository.Slug,
		Owner: repository.Project.Key,
	}
}

func (webhook *bitbucketServerWebhookParser) parsePrEvents(bitbucketCloudWebHook *bitbucketServerWebHook, event vcsutils.WebhookEvent) (*WebhookInfo, error) {
	eventTime, err := time.Parse("2006-01-02T15:04:05-0700", bitbucketCloudWebHook.Date)
	if err != nil {
		return nil, err
	}

	pullRequest := bitbucketCloudWebHook.PullRequest
	return &WebhookInfo{
		PullRequestId:           pullRequest.ID,
		TargetRepositoryDetails: webhook.getRepositoryDetails(pullRequest.ToRef.Repository),
		TargetBranch:            strings.TrimPrefix(pullRequest.ToRef.ID, "refs/heads/"),
		SourceRepositoryDetails: webhook.getRepositoryDetails(pullRequest.FromRef.Repository),
		SourceBranch:            strings.TrimPrefix(pullRequest.FromRef.ID, "refs/heads/"),
		Timestamp:               eventTime.UTC().Unix(),
		Event:                   event,
		PullRequest: &WebhookInfoPullRequest{
			ID:         pullRequest.ID,
			Title:      pullRequest.Title,
			CompareUrl: webhook.getFirstHrefFromLinks(pullRequest.Links) + "/diff",
			Timestamp:  pullRequest.UpdatedDate,
			Author: WebHookInfoUser{
				Login:       pullRequest.Author.User.Slug,
				DisplayName: pullRequest.Author.User.DisplayName,
				Email:       pullRequest.Author.User.EmailAddress,
				AvatarUrl:   webhook.getFirstHrefFromLinks(pullRequest.Author.User.Links),
			},
			TriggeredBy: WebHookInfoUser{
				Login: pullRequest.Author.User.Name,
			},
			SkipDecryption:   true,
			TargetRepository: webhook.getRepositoryDetails(pullRequest.ToRef.Repository),
			TargetBranch:     strings.TrimPrefix(pullRequest.ToRef.ID, "refs/heads/"),
			TargetHash:       pullRequest.ToRef.LatestCommit,
			SourceRepository: webhook.getRepositoryDetails(pullRequest.FromRef.Repository),
			SourceBranch:     strings.TrimPrefix(pullRequest.FromRef.ID, "refs/heads/"),
			SourceHash:       pullRequest.FromRef.LatestCommit,
		},
	}, nil
}

func (webhook *bitbucketServerWebhookParser) getFirstHrefFromLinks(links bitbucketv1.Links) string {
	if len(links.Self) > 0 {
		return links.Self[0].Href
	}
	return ""
}

func (webhook *bitbucketServerWebhookParser) branchStatus(to string, from string) WebHookInfoBranchStatus {
	existsAfter := to != gitNilHash
	existedBefore := from != gitNilHash
	return branchStatus(existedBefore, existsAfter)
}

type bitbucketServerWebHook struct {
	EventKey    string                          `json:"eventKey,omitempty"`
	Date        string                          `json:"date,omitempty"` // Timestamp
	Repository  bitbucketv1.Repository          `json:"repository,omitempty"`
	PullRequest bitbucketv1.PullRequest         `json:"pullRequest,omitempty"`
	Changes     []bitbucketServerWebHookChanges `json:"changes,omitempty"`
	Actor       bitbucketServerWebHookActor     `json:"actor,omitempty"`
}

type bitbucketServerWebHookChanges struct {
	Ref struct {
		DisplayId string `json:"displayId"`
		Type      string `json:"type"`
	}
	RefID    string `json:"refId,omitempty"`
	ToHash   string `json:"toHash,omitempty"`
	FromHash string `json:"fromHash,omitempty"`
	Type     string `json:"type"`
}

type bitbucketServerWebHookActor struct {
	Name         string            `json:"name,omitempty"`
	EmailAddress string            `json:"emailAddress,omitempty"`
	DisplayName  string            `json:"displayName,omitempty"`
	Links        bitbucketv1.Links `json:"links"`
}
