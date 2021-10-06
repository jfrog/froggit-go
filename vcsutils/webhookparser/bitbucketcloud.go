package webhookparser

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jfrog/froggit-go/vcsutils"
)

type BitbucketCloudWebhook struct {
	request *http.Request
}

func NewBitbucketCloudWebhookWebhook(request *http.Request) *BitbucketCloudWebhook {
	return &BitbucketCloudWebhook{
		request: request,
	}
}

func (webhook *BitbucketCloudWebhook) validatePayload(token []byte) ([]byte, error) {
	keys, tokenParamsExist := webhook.request.URL.Query()["token"]
	if len(token) > 0 || tokenParamsExist {
		if keys[0] != string(token) {
			return nil, errors.New("Token mismatch")
		}
	}
	payload := new(bytes.Buffer)
	if _, err := payload.ReadFrom(webhook.request.Body); err != nil {
		return nil, err
	}
	return payload.Bytes(), nil
}

func (webhook *BitbucketCloudWebhook) parseIncomingWebhook(payload []byte) (*WebhookInfo, error) {
	bitbucketCloudWebHook := &bitbucketCloudWebHook{}
	err := json.Unmarshal(payload, bitbucketCloudWebHook)
	if err != nil {
		return nil, err
	}

	event := webhook.request.Header.Get(EventHeaderKey)
	switch event {
	case "repo:push":
		return webhook.parsePushEvent(bitbucketCloudWebHook), nil
	case "pullrequest:created":
		return webhook.parsePrEvents(bitbucketCloudWebHook, vcsutils.PrCreated), nil
	case "pullrequest:updated":
		return webhook.parsePrEvents(bitbucketCloudWebHook, vcsutils.PrEdited), nil
	}
	return nil, nil
}

func (webhook *BitbucketCloudWebhook) parsePushEvent(bitbucketCloudWebHook *bitbucketCloudWebHook) *WebhookInfo {
	return &WebhookInfo{
		Repository: bitbucketCloudWebHook.Repository.FullName,
		Branch:     bitbucketCloudWebHook.Push.Changes[0].New.Name,
		Timestamp:  bitbucketCloudWebHook.Push.Changes[0].New.Target.Date.UTC().Unix(),
		Event:      vcsutils.Push,
	}
}

func (webhook *BitbucketCloudWebhook) parsePrEvents(bitbucketCloudWebHook *bitbucketCloudWebHook, event vcsutils.WebhookEvent) *WebhookInfo {
	return &WebhookInfo{
		Repository:       bitbucketCloudWebHook.PullRequest.Destination.Repository.FullName,
		Branch:           bitbucketCloudWebHook.PullRequest.Destination.Branch.Name,
		SourceRepository: bitbucketCloudWebHook.PullRequest.Source.Repository.FullName,
		SourceBranch:     bitbucketCloudWebHook.PullRequest.Source.Branch.Name,
		Timestamp:        bitbucketCloudWebHook.PullRequest.UpdatedOn.UTC().Unix(),
		Event:            event,
	}
}

type bitbucketCloudWebHook struct {
	Push struct {
		Changes []struct {
			New struct {
				Name   string `json:"name,omitempty"` // Branch name
				Target struct {
					Date time.Time `json:"date,omitempty"` // Timestamp
				} `json:"target,omitempty"`
			} `json:"new,omitempty"`
		} `json:"changes,omitempty"`
	} `json:"push,omitempty"`
	PullRequest struct {
		Source      struct{ bitbucketCloudPrRepository } `json:"source,omitempty"`
		Destination struct{ bitbucketCloudPrRepository } `json:"destination,omitempty"`
		UpdatedOn   time.Time                            `json:"updated_on,omitempty"` // Timestamp
	} `json:"pullrequest,omitempty"`
	Repository bitbucketCloudRepository `json:"repository,omitempty"`
}

type bitbucketCloudRepository struct {
	FullName string `json:"full_name,omitempty"` // Repository full name
}

type bitbucketCloudPrRepository struct {
	Repository bitbucketCloudRepository `json:"repository,omitempty"`
	Branch     struct {
		Name string `json:"name,omitempty"` // Branch name
	} `json:"branch,omitempty"`
}
