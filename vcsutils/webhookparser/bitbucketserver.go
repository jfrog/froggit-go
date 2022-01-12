package webhookparser

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/jfrog/froggit-go/vcsutils"
)

const sha256Signature = "X-Hub-Signature"
const bitbucketServerEventHeader = "X-Event-Key"

// BitbucketServerWebhook represents an incoming webhook on Bitbucket server
type BitbucketServerWebhook struct {
	request *http.Request
}

// NewBitbucketServerWebhookWebhook create a new BitbucketServerWebhook instance
func NewBitbucketServerWebhookWebhook(request *http.Request) *BitbucketServerWebhook {
	return &BitbucketServerWebhook{
		request: request,
	}
}

func (webhook *BitbucketServerWebhook) validatePayload(token []byte) ([]byte, error) {
	payload := new(bytes.Buffer)
	if _, err := payload.ReadFrom(webhook.request.Body); err != nil {
		return nil, err
	}

	expectedSignature := webhook.request.Header.Get(sha256Signature)
	if len(token) > 0 || len(expectedSignature) > 0 {
		actualSignature := calculatePayloadSignature(payload.Bytes(), token)
		if expectedSignature != "sha256="+actualSignature {
			return nil, errors.New("payload signature mismatch")
		}
	}
	return payload.Bytes(), nil
}

func (webhook *BitbucketServerWebhook) parseIncomingWebhook(payload []byte) (*WebhookInfo, error) {
	bitbucketServerWebHook := &bitbucketServerWebHook{}
	err := json.Unmarshal(payload, bitbucketServerWebHook)
	if err != nil {
		return nil, err
	}

	event := webhook.request.Header.Get(bitbucketServerEventHeader)
	switch event {
	case "repo:refs_changed":
		return webhook.parsePushEvent(bitbucketServerWebHook)
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

func (webhook *BitbucketServerWebhook) parsePushEvent(bitbucketCloudWebHook *bitbucketServerWebHook) (*WebhookInfo, error) {
	eventTime, err := time.Parse("2006-01-02T15:04:05-0700", bitbucketCloudWebHook.Date)
	if err != nil {
		return nil, err
	}
	repository := bitbucketCloudWebHook.Repository
	return &WebhookInfo{
		TargetRepositoryDetails: webhook.getRepositoryDetails(repository),
		TargetBranch:            strings.TrimPrefix(bitbucketCloudWebHook.Changes[0].RefID, "refs/heads/"),
		Timestamp:               eventTime.UTC().Unix(),
		Event:                   vcsutils.Push,
	}, nil
}

func (webhook *BitbucketServerWebhook) getRepositoryDetails(repository bitbucketv1.Repository) WebHookInfoRepoDetails {
	return WebHookInfoRepoDetails{
		Name:  repository.Slug,
		Owner: repository.Project.Key,
	}
}

func (webhook *BitbucketServerWebhook) parsePrEvents(bitbucketCloudWebHook *bitbucketServerWebHook, event vcsutils.WebhookEvent) (*WebhookInfo, error) {
	eventTime, err := time.Parse("2006-01-02T15:04:05-0700", bitbucketCloudWebHook.Date)
	if err != nil {
		return nil, err
	}
	return &WebhookInfo{
		PullRequestId:           bitbucketCloudWebHook.PullRequest.ID,
		TargetRepositoryDetails: webhook.getRepositoryDetails(bitbucketCloudWebHook.PullRequest.ToRef.Repository),
		TargetBranch:            strings.TrimPrefix(bitbucketCloudWebHook.PullRequest.ToRef.ID, "refs/heads/"),
		SourceRepositoryDetails: webhook.getRepositoryDetails(bitbucketCloudWebHook.PullRequest.FromRef.Repository),
		SourceBranch:            strings.TrimPrefix(bitbucketCloudWebHook.PullRequest.FromRef.ID, "refs/heads/"),
		Timestamp:               eventTime.UTC().Unix(),
		Event:                   event,
	}, nil
}

type bitbucketServerWebHook struct {
	EventKey    string                  `json:"eventKey,omitempty"`
	Date        string                  `json:"date,omitempty"` // Timestamp
	Repository  bitbucketv1.Repository  `json:"repository,omitempty"`
	PullRequest bitbucketv1.PullRequest `json:"pullRequest,omitempty"`
	Changes     []struct {
		RefID string `json:"refId,omitempty"`
	} `json:"changes,omitempty"`
}
