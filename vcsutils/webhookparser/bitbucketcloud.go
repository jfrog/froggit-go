package webhookparser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

// bitbucketCloudWebhookParser represents an incoming webhook on Bitbucket cloud
type bitbucketCloudWebhookParser struct {
	logger vcsclient.Log
}

// newBitbucketCloudWebhookParser create a new bitbucketCloudWebhookParser instance
func newBitbucketCloudWebhookParser(logger vcsclient.Log) *bitbucketCloudWebhookParser {
	return &bitbucketCloudWebhookParser{
		logger: logger,
	}
}

func (webhook *bitbucketCloudWebhookParser) validatePayload(_ context.Context, request *http.Request, token []byte) ([]byte, error) {
	keys, tokenParamsExist := request.URL.Query()["token"]
	if len(token) > 0 || tokenParamsExist {
		if keys[0] != string(token) {
			return nil, errors.New("token mismatch")
		}
	}
	payload := new(bytes.Buffer)
	if _, err := payload.ReadFrom(request.Body); err != nil {
		return nil, err
	}
	return payload.Bytes(), nil
}

func (webhook *bitbucketCloudWebhookParser) parseIncomingWebhook(_ context.Context, request *http.Request, payload []byte) (*WebhookInfo, error) {
	bitbucketCloudWebHook := &bitbucketCloudWebHook{}
	err := json.Unmarshal(payload, bitbucketCloudWebHook)
	if err != nil {
		return nil, err
	}

	event := request.Header.Get(EventHeaderKey)
	switch event {
	case "repo:push":
		return webhook.parsePushEvent(bitbucketCloudWebHook), nil
	case "pullrequest:created":
		return webhook.parsePrEvents(bitbucketCloudWebHook, vcsutils.PrOpened), nil
	case "pullrequest:updated":
		return webhook.parsePrEvents(bitbucketCloudWebHook, vcsutils.PrEdited), nil
	case "pullrequest:fulfilled":
		return webhook.parsePrEvents(bitbucketCloudWebHook, vcsutils.PrMerged), nil
	case "pullrequest:rejected":
		return webhook.parsePrEvents(bitbucketCloudWebHook, vcsutils.PrRejected), nil
	}
	return nil, nil
}

func (webhook *bitbucketCloudWebhookParser) parsePushEvent(bitbucketCloudWebHook *bitbucketCloudWebHook) *WebhookInfo {
	// In Push events, the hook provides a list of changes. Only the first one is relevant in our point of view.
	firstChange := bitbucketCloudWebHook.Push.Changes[0]
	lastCommit := firstChange.New.Target
	beforeCommitHash := webhook.parentOfLastCommit(lastCommit)
	return &WebhookInfo{
		TargetRepositoryDetails: webhook.parseRepoFullName(bitbucketCloudWebHook.Repository.FullName),
		TargetBranch:            webhook.branchName(firstChange),
		Timestamp:               lastCommit.Date.UTC().Unix(),
		Event:                   vcsutils.Push,
		Commit: WebHookInfoCommit{
			Hash:    lastCommit.Hash,
			Message: lastCommit.Message,
			Url:     lastCommit.Links.Html.Ref,
		},
		BeforeCommit: WebHookInfoCommit{
			Hash: beforeCommitHash,
		},
		BranchStatus: webhook.branchStatus(firstChange),
		TriggeredBy: WebHookInfoUser{
			Login: bitbucketCloudWebHook.Actor.Nickname,
		},
		Committer: WebHookInfoUser{
			Login: webhook.login(bitbucketCloudWebHook, lastCommit),
		},
		Author: WebHookInfoUser{
			Login: webhook.login(bitbucketCloudWebHook, lastCommit),
			Email: webhook.email(lastCommit),
		},
		CompareUrl: webhook.compareURL(bitbucketCloudWebHook, lastCommit, beforeCommitHash),
	}
}

// compareURL generates the HTML URL for the comparison between commits before and after push
func (webhook *bitbucketCloudWebhookParser) compareURL(bitbucketCloudWebHook *bitbucketCloudWebHook,
	lastCommit bitbucketCommit, beforeCommitHash string) string {
	if lastCommit.Hash == "" || beforeCommitHash == "" {
		return ""
	}
	return fmt.Sprintf("https://bitbucket.org/%s/branches/compare/%s..%s#diff",
		bitbucketCloudWebHook.Repository.FullName, lastCommit.Hash, beforeCommitHash)
}

// branchName gives the branch name a commit event refers to. It can be a branch that was created, updated or deleted.
func (webhook *bitbucketCloudWebhookParser) branchName(change bitbucketChange) string {
	branchName := change.New.Name
	if branchName == "" {
		branchName = change.Old.Name
	}
	return branchName
}

// email gives the email of a commit author. The email is actually contained in a raw string using RFC 5322 format
// e.g. "Barry Gibbs <bg@example.com>".
func (webhook *bitbucketCloudWebhookParser) email(lastCommit bitbucketCommit) string {
	email := lastCommit.Author.Raw
	parsedEmail, err := mail.ParseAddress(lastCommit.Author.Raw)
	if err == nil && parsedEmail != nil {
		email = parsedEmail.Address
	}
	return email
}

func (webhook *bitbucketCloudWebhookParser) parsePrEvents(bitbucketCloudWebHook *bitbucketCloudWebHook, event vcsutils.WebhookEvent) *WebhookInfo {
	return &WebhookInfo{
		PullRequestId:           bitbucketCloudWebHook.PullRequest.ID,
		TargetRepositoryDetails: webhook.parseRepoFullName(bitbucketCloudWebHook.PullRequest.Destination.Repository.FullName),
		TargetBranch:            bitbucketCloudWebHook.PullRequest.Destination.Branch.Name,
		SourceRepositoryDetails: webhook.parseRepoFullName(bitbucketCloudWebHook.PullRequest.Source.Repository.FullName),
		SourceBranch:            bitbucketCloudWebHook.PullRequest.Source.Branch.Name,
		Timestamp:               bitbucketCloudWebHook.PullRequest.UpdatedOn.UTC().Unix(),
		Event:                   event,
	}
}

func (webhook *bitbucketCloudWebhookParser) parseRepoFullName(fullName string) WebHookInfoRepoDetails {
	// From https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#Repository
	// "full_name : The workspace and repository slugs joined with a '/'."
	split := strings.Split(fullName, "/")
	return WebHookInfoRepoDetails{
		Name:  split[1],
		Owner: split[0],
	}
}

func (webhook *bitbucketCloudWebhookParser) parentOfLastCommit(lastCommit bitbucketCommit) string {
	if len(lastCommit.Parents) == 0 {
		return ""
	}
	return lastCommit.Parents[0].Hash
}

// login gets the username of a commit author.
func (webhook *bitbucketCloudWebhookParser) login(hook *bitbucketCloudWebHook, lastCommit bitbucketCommit) string {
	if lastCommit.Author.User.Nickname != "" {
		return lastCommit.Author.User.Nickname
	}
	return hook.Actor.Nickname
}

func (webhook *bitbucketCloudWebhookParser) branchStatus(change bitbucketChange) WebHookInfoBranchStatus {
	existsAfter := change.New.Name != ""
	existedBefore := change.Old.Name != ""
	return branchStatus(existedBefore, existsAfter)
}

type bitbucketCloudWebHook struct {
	Push        bitbucketPush            `json:"push,omitempty"`
	PullRequest bitbucketPullRequest     `json:"pullrequest,omitempty"`
	Repository  bitbucketCloudRepository `json:"repository,omitempty"`
	Actor       struct {
		Nickname string `json:"nickname,omitempty"`
	} `json:"actor,omitempty"`
}

type bitbucketPullRequest struct {
	ID          int                        `json:"id,omitempty"`
	Source      bitbucketCloudPrRepository `json:"source,omitempty"`
	Destination bitbucketCloudPrRepository `json:"destination,omitempty"`
	UpdatedOn   time.Time                  `json:"updated_on,omitempty"`
}

type bitbucketPush struct {
	Changes []bitbucketChange `json:"changes,omitempty"`
}

type bitbucketChange struct {
	New struct {
		// Name is the new branch name
		Name   string          `json:"name,omitempty"`
		Target bitbucketCommit `json:"target,omitempty"`
	} `json:"new,omitempty"`
	Old struct {
		// Name is the old branch name
		Name string `json:"name,omitempty"`
	} `json:"old,omitempty"`
}

type bitbucketCommit struct {
	Date    time.Time `json:"date,omitempty"`    // Timestamp
	Hash    string    `json:"hash,omitempty"`    // Commit Hash
	Message string    `json:"message,omitempty"` // Commit message
	Author  struct {
		Raw  string `json:"raw,omitempty"` // Commit author
		User struct {
			Nickname string `json:"nickname,omitempty"`
		} `json:"user,omitempty"`
	} `json:"author,omitempty"`
	Links struct {
		Html struct {
			Ref string `json:"ref,omitempty"` // Commit URL
		} `json:"html,omitempty"`
	} `json:"links,omitempty"`
	Parents []struct {
		Hash string `json:"hash,omitempty"` // Commit Hash
	} `json:"parents,omitempty"`
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
