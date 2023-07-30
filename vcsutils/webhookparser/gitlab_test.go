package webhookparser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xanzy/go-gitlab"

	"github.com/stretchr/testify/assert"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

const (
	gitLabEventHeader          = "X-GitLab-Event"
	gitlabPushExpectedTime     = int64(1630306883)
	gitlabPrOpenExpectedTime   = int64(1631202047)
	gitlabPrReopenExpectedTime = int64(1638865856)
	gitlabPrUpdateExpectedTime = int64(1631202266)
	gitlabPrCloseExpectedTime  = int64(1638864453)
	gitlabPrMergeExpectedTime  = int64(1638866119)
	gitlabExpectedPrID         = 1
)

func TestGitLabParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "pushpayload.json"))
	assert.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, string(token))
	request.Header.Add(gitLabEventHeader, "Push Hook")

	// Parse webhook
	actual, err := ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.GitLab,
			Token:       token,
		}, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, expectedOwner, actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, gitlabPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
	assert.Equal(t, WebHookInfoUser{DisplayName: "Yahav Itzhak", Email: "yahavitz@gmail.com"}, actual.Author)
	assert.Equal(t, WebHookInfoUser{DisplayName: "Yahav Itzhak", Email: "yahavitz@gmail.com"}, actual.Committer)
	assert.Equal(t, WebHookInfoUser{Login: "yahavi", DisplayName: "Yahav Itzhak", AvatarUrl: "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?s=80&d=identicon"}, actual.TriggeredBy)
	assert.Equal(t, WebHookInfoCommit{
		Hash:    "450cd4687e3644d544ca4cb3a7a355fea9e6f0dc",
		Message: "Initial commit",
		Url:     "https://gitlab.com/yahavi/hello-world/-/commit/450cd4687e3644d544ca4cb3a7a355fea9e6f0dc",
	}, actual.Commit)
	assert.Equal(t, WebHookInfoCommit{
		Hash: "450cd4687e3644d544ca4cb3a7a355fea9e6f0dc",
	}, actual.BeforeCommit)
	assert.Equal(t, WebhookInfoBranchStatusUpdated, actual.BranchStatus)
	assert.Equal(t, "", actual.CompareUrl)
}

func TestGitLabParseIncomingPrWebhook(t *testing.T) {
	tests := []struct {
		name                    string
		payloadFilename         string
		expectedTime            int64
		expectedEventType       vcsutils.WebhookEvent
		expectedPullRequestInfo *WebhookInfoPullRequest
	}{
		{
			name:              "open",
			payloadFilename:   "propenpayload.json",
			expectedTime:      gitlabPrOpenExpectedTime,
			expectedEventType: vcsutils.PrOpened,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         1,
				Title:      "Update README.md",
				CompareUrl: "https://gitlab.com/yahavi/hello-world/-/merge_requests/1",
				Timestamp:  1631202047,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					Email:       "yahavitz@gmail.com",
					AvatarUrl:   "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?s=80&d=identicon",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
					Email: "yahavitz@gmail.com",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "72108853aa0eac9d1b72fe34710aeed256d193d5",
			},
		},
		{
			name:              "reopen",
			payloadFilename:   "prreopenpayload.json",
			expectedTime:      gitlabPrReopenExpectedTime,
			expectedEventType: vcsutils.PrOpened,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         1,
				Title:      "Update README.md",
				CompareUrl: "https://gitlab.com/yahavi/hello-world/-/merge_requests/1",
				Timestamp:  1638865856,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					Email:       "yahavitz@gmail.com",
					AvatarUrl:   "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?s=80&d=identicon",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
					Email: "yahavitz@gmail.com",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "3fcd302505fb3df664143df4ddcb6cfc50ff2ea8",
			},
		},
		{
			name:              "update",
			payloadFilename:   "prupdatepayload.json",
			expectedTime:      gitlabPrUpdateExpectedTime,
			expectedEventType: vcsutils.PrEdited,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         1,
				Title:      "Update README.md",
				CompareUrl: "https://gitlab.com/yahavi/hello-world/-/merge_requests/1",
				Timestamp:  1631202266,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					Email:       "yahavitz@gmail.com",
					AvatarUrl:   "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?s=80&d=identicon",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
					Email: "yahavitz@gmail.com",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "3fcd302505fb3df664143df4ddcb6cfc50ff2ea8",
			},
		},
		{
			name:              "close",
			payloadFilename:   "prclosepayload.json",
			expectedTime:      gitlabPrCloseExpectedTime,
			expectedEventType: vcsutils.PrRejected,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         1,
				Title:      "Update README.md",
				CompareUrl: "https://gitlab.com/yahavi/hello-world/-/merge_requests/1",
				Timestamp:  1638864453,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					Email:       "yahavitz@gmail.com",
					AvatarUrl:   "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?s=80&d=identicon",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
					Email: "yahavitz@gmail.com",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "3fcd302505fb3df664143df4ddcb6cfc50ff2ea8",
			},
		},
		{
			name:              "merge",
			payloadFilename:   "prmergepayload.json",
			expectedTime:      gitlabPrMergeExpectedTime,
			expectedEventType: vcsutils.PrMerged,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         1,
				Title:      "Update README.md",
				CompareUrl: "https://gitlab.com/yahavi/hello-world/-/merge_requests/1",
				Timestamp:  1638866119,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					Email:       "yahavitz@gmail.com",
					AvatarUrl:   "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?s=80&d=identicon",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
					Email: "yahavitz@gmail.com",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "3fcd302505fb3df664143df4ddcb6cfc50ff2ea8",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := os.Open(filepath.Join("testdata", "gitlab", tt.payloadFilename))
			assert.NoError(t, err)
			defer close(reader)

			// Create request
			request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
			request.Header.Add(gitLabKeyHeader, string(token))
			request.Header.Add(gitLabEventHeader, "Merge Request Hook")

			// Parse webhook
			actual, err := ParseIncomingWebhook(context.Background(),
				vcsclient.EmptyLogger{},
				WebhookOrigin{
					VcsProvider: vcsutils.GitLab,
					Token:       token,
				}, request)
			assert.NoError(t, err)

			// Check values
			assert.Equal(t, gitlabExpectedPrID, actual.PullRequestId)
			assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
			assert.Equal(t, expectedOwner, actual.TargetRepositoryDetails.Owner)
			assert.Equal(t, expectedBranch, actual.TargetBranch)
			assert.Equal(t, tt.expectedTime, actual.Timestamp)
			assert.Equal(t, expectedRepoName, actual.SourceRepositoryDetails.Name)
			assert.Equal(t, expectedOwner, actual.SourceRepositoryDetails.Owner)
			assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
			assert.Equal(t, tt.expectedEventType, actual.Event)
			assert.Equal(t, tt.expectedPullRequestInfo, actual.PullRequest)
		})
	}
}

func TestGitLabParseIncomingWebhookTagEvents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		payloadFilename   string
		expectedEventType vcsutils.WebhookEvent
		expectedTagInfo   *WebhookInfoTag
	}{
		{
			name:              "created",
			payloadFilename:   "tagcreatepayload.json",
			expectedEventType: vcsutils.TagPushed,
			expectedTagInfo: &WebhookInfoTag{
				Name:       "first_tag",
				Hash:       "45abefa4485f0e03fa5db86a998d16a0a1df07b7",
				TargetHash: "0497394e95db76bd27a177955694e06987da47e7",
				Message:    "This is a first tag",
				Repository: WebHookInfoRepoDetails{
					Name:  "webhooktest",
					Owner: "grobinov",
				},
				Author: WebHookInfoUser{
					Login:       "robinov",
					DisplayName: "Rob Ivanov",
					AvatarUrl:   "https://secure.gravatar.com/avatar/a47aec620663eaa2bfeaa0540e580fd5?s=80&d=identicon",
				},
			},
		},
		{
			name:              "deleted",
			payloadFilename:   "tagdeletepayload.json",
			expectedEventType: vcsutils.TagRemoved,
			expectedTagInfo: &WebhookInfoTag{
				Name: "first_tag",
				Hash: "45abefa4485f0e03fa5db86a998d16a0a1df07b7",
				Repository: WebHookInfoRepoDetails{
					Name:  "webhooktest",
					Owner: "grobinov",
				},
				Author: WebHookInfoUser{
					Login:       "robinov",
					DisplayName: "Rob Ivanov",
					AvatarUrl:   "https://secure.gravatar.com/avatar/a47aec620663eaa2bfeaa0540e580fd5?s=80&d=identicon",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := os.Open(filepath.Join("testdata", "gitlab", tt.payloadFilename))
			assert.NoError(t, err)
			defer close(reader)

			request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
			request.Header.Add(gitLabKeyHeader, string(token))
			request.Header.Add(gitLabEventHeader, string(gitlab.EventTypeTagPush))

			actual, err := ParseIncomingWebhook(
				context.Background(),
				vcsclient.EmptyLogger{},
				WebhookOrigin{
					VcsProvider: vcsutils.GitLab,
					Token:       token,
				},
				request,
			)
			assert.NoError(t, err)
			assert.Equal(t, &WebhookInfo{Event: tt.expectedEventType, Tag: tt.expectedTagInfo}, actual)
		})
	}
}

func TestGitLabParseIncomingWebhookError(t *testing.T) {
	request := &http.Request{}
	_, err := ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.GitLab,
			Token:       token,
		}, request)

	assert.Error(t, err)

	webhook := gitLabWebhookParser{logger: vcsclient.EmptyLogger{}}
	_, err = webhook.parseIncomingWebhook(context.Background(), request, []byte{})
	assert.Error(t, err)
}

func TestGitLabParsePrEventsError(t *testing.T) {
	webhook := gitLabWebhookParser{}
	webhookInfo, _ := webhook.parsePrEvents(&gitlab.MergeEvent{})
	assert.Nil(t, webhookInfo)
}

func TestGitLabPayloadMismatchSignature(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "pushpayload.json"))
	assert.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, "wrong-token")
	request.Header.Add(gitLabEventHeader, "Push Hook")

	// Parse webhook
	_, err = ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.GitLab,
			Token:       token,
		}, request)
	assert.EqualError(t, err, "token mismatch")
}
