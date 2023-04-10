package webhookparser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

const (
	bitbucketCloudPushExpectedTime     = int64(1630824565)
	bitbucketCloudPrCreateExpectedTime = int64(1630831665)
	bitbucketCloudPrUpdateExpectedTime = int64(1630844170)
	bitbucketCloudPrMergeExpectedTime  = int64(1638783257)
	bitbucketCloudPrCloseExpectedTime  = int64(1638784487)
	bitbucketCloudExpectedPrID         = 2
)

func TestBitbucketCloudParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketcloud", "pushpayload.json"))
	require.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1?token="+string(token), reader)
	request.Header.Add(EventHeaderKey, "repo:push")

	// Parse webhook
	actual, err := ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.BitbucketCloud,
			Token:       token,
		},
		request)
	require.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, expectedOwner, actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, bitbucketCloudPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
	assert.Equal(t, WebHookInfoUser{Login: "yahavi", Email: "yahavitz@gmail.com"}, actual.Author)
	assert.Equal(t, WebHookInfoUser{Login: "yahavi"}, actual.Committer)
	assert.Equal(t, WebHookInfoUser{Login: "yahavi"}, actual.TriggeredBy)
	assert.Equal(t, WebHookInfoCommit{
		Hash:    "fa8c303777d0006fa99b843b830ad1ed18a6928e",
		Message: "README.md edited online with Bitbucket",
	}, actual.Commit)
	assert.Equal(t, WebHookInfoCommit{
		Hash: "a2b4032ae25e08844b894e413d80ee75b4c1995b",
	}, actual.BeforeCommit)
	assert.Equal(t, WebhookInfoBranchStatusUpdated, actual.BranchStatus)
	assert.Equal(t, "https://bitbucket.org/yahavi/hello-world/branches/compare/fa8c303777d0006fa99b843b830ad1ed18a6928e..a2b4032ae25e08844b894e413d80ee75b4c1995b#diff", actual.CompareUrl)
}

func TestBitbucketCloudParseIncomingPrWebhook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		payloadFilename     string
		eventHeader         string
		expectedTime        int64
		expectedEventType   vcsutils.WebhookEvent
		expectedPullRequest *WebhookInfoPullRequest
	}{
		{
			name:              "create",
			payloadFilename:   "prcreatepayload.json",
			eventHeader:       "pullrequest:created",
			expectedTime:      bitbucketCloudPrCreateExpectedTime,
			expectedEventType: vcsutils.PrOpened,
			expectedPullRequest: &WebhookInfoPullRequest{
				ID:         bitbucketCloudExpectedPrID,
				Title:      "Dev",
				CompareUrl: "https://bitbucket.org/yahavi/hello-world/pull-requests/2",
				Timestamp:  1630831665,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					AvatarUrl: "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?d=https%3A%2F%2F" +
						"avatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYI-5.png",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "fa8c303777d0",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "363994ee7c2e",
			},
		},
		{
			name:              "update",
			payloadFilename:   "prupdatepayload.json",
			eventHeader:       "pullrequest:updated",
			expectedTime:      bitbucketCloudPrUpdateExpectedTime,
			expectedEventType: vcsutils.PrEdited,
			expectedPullRequest: &WebhookInfoPullRequest{
				ID:         bitbucketCloudExpectedPrID,
				Title:      "Dev",
				CompareUrl: "https://bitbucket.org/yahavi/hello-world/pull-requests/2",
				Timestamp:  1630844170,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					AvatarUrl: "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?d=https%3A%2F%2F" +
						"avatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYI-5.png",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "78c315141e83",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "1147000c51dc",
			},
		},
		{
			name:              "merge",
			payloadFilename:   "prmergepayload.json",
			eventHeader:       "pullrequest:fulfilled",
			expectedTime:      bitbucketCloudPrMergeExpectedTime,
			expectedEventType: vcsutils.PrMerged,
			expectedPullRequest: &WebhookInfoPullRequest{
				ID:         bitbucketCloudExpectedPrID,
				Title:      "Dev",
				CompareUrl: "https://bitbucket.org/yahavi/hello-world/pull-requests/2",
				Timestamp:  1638783257,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					AvatarUrl: "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?d=https%3A%2F%2F" +
						"avatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYI-5.png",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "78c315141e83",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "1147000c51dc",
			},
		},
		{
			name:              "close",
			payloadFilename:   "prclosepayload.json",
			eventHeader:       "pullrequest:rejected",
			expectedTime:      bitbucketCloudPrCloseExpectedTime,
			expectedEventType: vcsutils.PrRejected,
			expectedPullRequest: &WebhookInfoPullRequest{
				ID:         bitbucketCloudExpectedPrID,
				Title:      "Dev",
				CompareUrl: "https://bitbucket.org/yahavi/hello-world/pull-requests/2",
				Timestamp:  1638784487,
				Author: WebHookInfoUser{
					Login:       "yahavi",
					DisplayName: "Yahav Itzhak",
					AvatarUrl: "https://secure.gravatar.com/avatar/9680da1674e22a1de17acb19bb233ebf?d=https%3A%2F%2F" +
						"avatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYI-5.png",
				},
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption: true,
				TargetRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				TargetBranch: "main",
				TargetHash:   "78c315141e83",
				SourceRepository: WebHookInfoRepoDetails{
					Name:  "hello-world",
					Owner: "yahavi",
				},
				SourceBranch: "dev",
				SourceHash:   "1147000c51dc",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := os.Open(filepath.Join("testdata", "bitbucketcloud", tt.payloadFilename))
			require.NoError(t, err)
			defer close(reader)

			// Create request
			request := httptest.NewRequest("POST", "https://127.0.0.1?token="+string(token), reader)
			request.Header.Add(EventHeaderKey, tt.eventHeader)

			// Parse webhook
			actual, err := ParseIncomingWebhook(context.Background(),
				vcsclient.EmptyLogger{},
				WebhookOrigin{
					VcsProvider: vcsutils.BitbucketCloud,
					Token:       token,
				},
				request)
			require.NoError(t, err)

			// Check values
			assert.Equal(t, bitbucketCloudExpectedPrID, actual.PullRequestId)
			assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
			assert.Equal(t, expectedOwner, actual.TargetRepositoryDetails.Owner)
			assert.Equal(t, expectedBranch, actual.TargetBranch)
			assert.Equal(t, tt.expectedTime, actual.Timestamp)
			assert.Equal(t, expectedRepoName, actual.SourceRepositoryDetails.Name)
			assert.Equal(t, expectedOwner, actual.SourceRepositoryDetails.Owner)
			assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
			assert.Equal(t, tt.expectedEventType, actual.Event)
			assert.Equal(t, tt.expectedPullRequest, actual.PullRequest)
		})
	}
}

func TestBitbucketCloudParseIncomingWebhookError(t *testing.T) {
	request := &http.Request{URL: &url.URL{RawQuery: "token=a"}}
	_, err := ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.BitbucketCloud,
			Token:       token,
		},
		request)
	assert.Error(t, err)

	webhook := bitbucketCloudWebhookParser{}
	_, err = webhook.parseIncomingWebhook(context.Background(), request, []byte{})
	assert.Error(t, err)
}

func TestBitbucketCloudPayloadMismatchToken(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketcloud", "pushpayload.json"))
	require.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1?token=wrong-token", reader)
	request.Header.Add(EventHeaderKey, "repo:push")

	// Parse webhook
	_, err = ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.BitbucketCloud,
			Token:       token,
		},
		request)
	assert.EqualError(t, err, "token mismatch")
}
