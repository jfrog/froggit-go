package webhookparser

import (
	"bytes"
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

	"github.com/jfrog/froggit-go/vcsutils"
)

const sha256Signature = "X-Hub-Signature"
const bitbucketServerEventHeader = "X-Event-Key"

// BitbucketServerWebhook represents an incoming webhook on Bitbucket server
type BitbucketServerWebhook struct {
	request  *http.Request
	endpoint string
}

type BitbucketServerWebhookOption func(*BitbucketServerWebhook)

func WithBitbucketServerEndpoint(endpoint string) BitbucketServerWebhookOption {
	return func(w *BitbucketServerWebhook) {
		if endpoint != "" && strings.HasSuffix(endpoint, "/") {
			w.endpoint = endpoint[:len(endpoint)-1]
		} else {
			w.endpoint = endpoint
		}
	}
}

// NewBitbucketServerWebhookWebhook create a new BitbucketServerWebhook instance
func NewBitbucketServerWebhookWebhook(request *http.Request, options ...BitbucketServerWebhookOption) *BitbucketServerWebhook {
	w := &BitbucketServerWebhook{
		request:  request,
		endpoint: "undefined",
	}
	for i := range options {
		options[i](w)
	}
	return w
}

func (webhook *BitbucketServerWebhook) Parse(token []byte) (*WebhookInfo, error) {
	return validateAndParseHttpRequest(webhook, token, webhook.request)
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
	repositoryDetails := webhook.getRepositoryDetails(repository)
	return &WebhookInfo{
		TargetRepositoryDetails: repositoryDetails,
		TargetBranch:            strings.TrimPrefix(bitbucketCloudWebHook.Changes[0].RefID, "refs/heads/"),
		PullRequestId:           0,                        // unused for push event
		SourceRepositoryDetails: WebHookInfoRepoDetails{}, // unused for push event
		SourceBranch:            "",                       // unused for push event
		Timestamp:               eventTime.UTC().Unix(),
		Event:                   vcsutils.Push,
		Commit: WebHookInfoCommit{
			Hash:    bitbucketCloudWebHook.Changes[0].ToHash,
			Message: "",
			Url: fmt.Sprintf("%s/projects/%s/repos/%s/commits/%s", webhook.endpoint,
				repositoryDetails.Owner, repositoryDetails.Name, bitbucketCloudWebHook.Changes[0].ToHash),
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
		CompareUrl: "",
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

func (webhook *BitbucketServerWebhook) branchStatus(to string, from string) WebHookInfoBranchStatus {
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
	RefID    string `json:"refId,omitempty"`
	ToHash   string `json:"toHash,omitempty"`
	FromHash string `json:"fromHash,omitempty"`
}

type bitbucketServerWebHookActor struct {
	Name         string `json:"name,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
}
