package webhookparser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jfrog/froggit-go/vcsclient"
	"github.com/jfrog/froggit-go/vcsutils"
)

const (
	bitbucketServerPushSha256       = "726b95677f1eeecc07acce435b9d29d7360242e171bbe70a5db811bcb37ef039"
	bitbucketServerPushExpectedTime = int64(1631178392)

	bitbucketServerPrCreateExpectedTime = int64(1631178661)
	bitbucketServerPrCreatedSha256      = "0f7e43b2c1593777bca7f1e4e55a183ba3e982409a6fc6f3a5bdc0304de320af"

	bitbucketServerPrUpdateExpectedTime = int64(1631180185)
	bitbucketServerPrUpdatedSha256      = "a7314de684499eef16bd781af5367f70a02307c1894a25265adfccb2b5bbabbe"

	bitbucketServerPrMergeExpectedTime = int64(1638794461)
	bitbucketServerPrMergedSha256      = "21434ba0f4b6fd9abd2238173a41157b0479ccdb491e325182dcf18d6598a9b2"

	bitbucketServerPrDeclineExpectedTime = int64(1638794521)
	bitbucketServerPrDeclinedSha256      = "7e09bf49383183c10b46e6a3c3e9a73cc3bcda2b4a5a8c93aad96552c0262ce6"

	bitbucketServerPrDeleteExpectedTime = int64(1638794581)
	bitbucketServerPrDeletedSha256      = "b0ccbd0f97ca030aa469cfa559f7051732c33fc63e7e3a8b5b8e2d157af71806"

	bitbucketServerTagPushedSha256  = "f55f6b6317b24cd19c21876db1832460978b5c6376ba6ce740b8649c1bcd41e0"
	bitbucketServerTagRemovedSha256 = "0c951396d86b353de850d49bd76556f2b2336c3ff4f437086113f44be5421bb9"

	bitbucketServerExpectedPrID = 3
)

func TestBitbucketServerParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "pushpayload.json"))
	assert.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
	request.Header.Add(EventHeaderKey, "repo:refs_changed")
	request.Header.Add(sha256Signature, "sha256="+bitbucketServerPushSha256)

	// Parse webhook
	parser := newBitbucketServerWebhookParser(vcsclient.EmptyLogger{}, "https://bitbucket.test/rest")
	actual, err := validateAndParseHttpRequest(context.Background(), parser, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, bitbucketServerPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
	assert.Equal(t, WebHookInfoUser{DisplayName: "Yahav Itzhak", Email: "yahavi@jfrog.com"}, actual.Author)
	assert.Equal(t, WebHookInfoUser{DisplayName: "Yahav Itzhak", Email: "yahavi@jfrog.com"}, actual.Committer)
	assert.Equal(t, WebHookInfoUser{Login: "yahavi", DisplayName: "Yahav Itzhak"}, actual.TriggeredBy)
	assert.Equal(t, WebHookInfoCommit{
		Hash:    "929d3054cf60e11a38672966f948bb5d95f48f0e",
		Message: "",
		Url:     "https://bitbucket.test/rest/projects/~YAHAVI/repos/hello-world/commits/929d3054cf60e11a38672966f948bb5d95f48f0e",
	}, actual.Commit)
	assert.Equal(t, WebHookInfoCommit{
		Hash: "0000000000000000000000000000000000000000",
	}, actual.BeforeCommit)
	assert.Equal(t, WebhookInfoBranchStatusCreated, actual.BranchStatus)
	assert.Equal(t, "", actual.CompareUrl)
}

func TestBitbucketServerParseIncomingPrWebhook(t *testing.T) {
	author := WebHookInfoUser{
		Login:       "yahavi",
		DisplayName: "Yahav Itzhak",
		Email:       "yahavi@jfrog.com",
		AvatarUrl:   "https://git.acme.info/users/yahavi",
	}

	repository := WebHookInfoRepoDetails{
		Name:  "hello-world",
		Owner: "~YAHAVI",
	}

	const title = "Update README.md"
	const href = "https://git.acme.info/users/yahavi/repos/hello-world/pull-requests/3"
	tests := []struct {
		name                    string
		payloadFilename         string
		eventHeader             string
		payloadSha              string
		expectedTime            int64
		expectedEventType       vcsutils.WebhookEvent
		expectedPullRequestInfo *WebhookInfoPullRequest
	}{
		{
			name:              "create",
			payloadFilename:   "prcreatepayload.json",
			eventHeader:       "pr:opened",
			payloadSha:        bitbucketServerPrCreatedSha256,
			expectedTime:      bitbucketServerPrCreateExpectedTime,
			expectedEventType: vcsutils.PrOpened,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         bitbucketServerExpectedPrID,
				Title:      title,
				CompareUrl: href + "/diff",
				Timestamp:  1631178661307,
				Author:     author,
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption:   true,
				TargetRepository: repository,
				TargetBranch:     "main",
				TargetHash:       "929d3054cf60e11a38672966f948bb5d95f48f0e",
				SourceRepository: repository,
				SourceBranch:     "dev",
				SourceHash:       "b3fc2f0a02761b443fca72022a2ac897cc2ceb3a",
			},
		},
		{
			name:              "update",
			payloadFilename:   "prupdatepayload.json",
			eventHeader:       "pr:from_ref_updated",
			payloadSha:        bitbucketServerPrUpdatedSha256,
			expectedTime:      bitbucketServerPrUpdateExpectedTime,
			expectedEventType: vcsutils.PrEdited,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         bitbucketServerExpectedPrID,
				Title:      title,
				CompareUrl: href + "/diff",
				Timestamp:  1631180185186,
				Author:     author,
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption:   true,
				TargetRepository: repository,
				TargetBranch:     "main",
				TargetHash:       "929d3054cf60e11a38672966f948bb5d95f48f0e",
				SourceRepository: repository,
				SourceBranch:     "dev",
				SourceHash:       "4116e7fd0dffeff395c4697653912ad4c19e1c5b",
			},
		},
		{
			name:              "merge",
			payloadFilename:   "prmergepayload.json",
			eventHeader:       "pr:merged",
			payloadSha:        bitbucketServerPrMergedSha256,
			expectedTime:      bitbucketServerPrMergeExpectedTime,
			expectedEventType: vcsutils.PrMerged,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         bitbucketServerExpectedPrID,
				Title:      title,
				CompareUrl: href + "/diff",
				Timestamp:  1638794461247,
				Author:     author,
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption:   true,
				TargetRepository: repository,
				TargetBranch:     "main",
				TargetHash:       "929d3054cf60e11a38672966f948bb5d95f48f0e",
				SourceRepository: repository,
				SourceBranch:     "dev",
				SourceHash:       "b3fc2f0a02761b443fca72022a2ac897cc2ceb3a",
			},
		},
		{
			name:              "decline",
			payloadFilename:   "prdeclinepayload.json",
			eventHeader:       "pr:declined",
			payloadSha:        bitbucketServerPrDeclinedSha256,
			expectedTime:      bitbucketServerPrDeclineExpectedTime,
			expectedEventType: vcsutils.PrRejected,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         bitbucketServerExpectedPrID,
				Title:      title,
				CompareUrl: href + "/diff",
				Timestamp:  1638794521247,
				Author:     author,
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption:   true,
				TargetRepository: repository,
				TargetBranch:     "main",
				TargetHash:       "929d3054cf60e11a38672966f948bb5d95f48f0e",
				SourceRepository: repository,
				SourceBranch:     "dev",
				SourceHash:       "b3fc2f0a02761b443fca72022a2ac897cc2ceb3a",
			},
		},
		{
			name:              "delete",
			payloadFilename:   "prdeletepayload.json",
			eventHeader:       "pr:deleted",
			payloadSha:        bitbucketServerPrDeletedSha256,
			expectedTime:      bitbucketServerPrDeleteExpectedTime,
			expectedEventType: vcsutils.PrRejected,
			expectedPullRequestInfo: &WebhookInfoPullRequest{
				ID:         bitbucketServerExpectedPrID,
				Title:      title,
				CompareUrl: href + "/diff",
				Timestamp:  1638794581247,
				Author:     author,
				TriggeredBy: WebHookInfoUser{
					Login: "yahavi",
				},
				SkipDecryption:   true,
				TargetRepository: repository,
				TargetBranch:     "main",
				TargetHash:       "929d3054cf60e11a38672966f948bb5d95f48f0e",
				SourceRepository: repository,
				SourceBranch:     "dev",
				SourceHash:       "b3fc2f0a02761b443fca72022a2ac897cc2ceb3a",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", tt.payloadFilename))
			assert.NoError(t, err)
			defer close(reader)

			// Create request
			request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
			request.Header.Add(EventHeaderKey, tt.eventHeader)
			request.Header.Add(sha256Signature, "sha256="+tt.payloadSha)

			// Parse webhook
			actual, err := ParseIncomingWebhook(context.Background(),
				vcsclient.EmptyLogger{},
				WebhookOrigin{
					VcsProvider: vcsutils.BitbucketServer,
					Token:       token,
				},
				request)
			assert.NoError(t, err)

			// Check values
			assert.Equal(t, bitbucketServerExpectedPrID, actual.PullRequestId)
			assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
			assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.TargetRepositoryDetails.Owner)
			assert.Equal(t, expectedBranch, actual.TargetBranch)
			assert.Equal(t, tt.expectedTime, actual.Timestamp)
			assert.Equal(t, expectedRepoName, actual.SourceRepositoryDetails.Name)
			assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.SourceRepositoryDetails.Owner)
			assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
			assert.Equal(t, tt.expectedEventType, actual.Event)
			assert.Equal(t, tt.expectedPullRequestInfo, actual.PullRequest)
		})
	}
}
func TestBitbucketServerParseIncomingWebhookTagEvents(t *testing.T) {
	t.Parallel()
	author := WebHookInfoUser{
		Login:       "pavlos",
		DisplayName: "Pavlo Strokov[EXT]",
		Email:       "pavlos@jfrog.com",
		AvatarUrl:   "https://git.jfrog.info/users/pavlos",
	}

	repository := WebHookInfoRepoDetails{
		Name:  "integration-test",
		Owner: "~PAVLOS",
	}

	tests := []struct {
		name              string
		payloadFilename   string
		eventHeader       string
		payloadSha        string
		expectedEventType vcsutils.WebhookEvent
		expectedTagInfo   *WebhookInfoTag
	}{
		{
			name:              "created",
			payloadFilename:   "tagcreatepayload.json",
			eventHeader:       "repo:refs_changed",
			payloadSha:        bitbucketServerTagPushedSha256,
			expectedEventType: vcsutils.TagPushed,
			expectedTagInfo: &WebhookInfoTag{
				Name:       "tag_intg",
				Hash:       "32e1a97a1a09a735ab77b4f0fb26cb5550cc2713",
				Repository: repository,
				Author:     author,
			},
		},
		{
			name:              "deleted",
			payloadFilename:   "tagdeletepayload.json",
			eventHeader:       "repo:refs_changed",
			payloadSha:        bitbucketServerTagRemovedSha256,
			expectedEventType: vcsutils.TagRemoved,
			expectedTagInfo: &WebhookInfoTag{
				Name:       "tag_intg",
				Hash:       "32e1a97a1a09a735ab77b4f0fb26cb5550cc2713",
				Repository: repository,
				Author:     author,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", tt.payloadFilename))
			assert.NoError(t, err)
			defer close(reader)

			request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
			request.Header.Add(EventHeaderKey, tt.eventHeader)
			request.Header.Add(sha256Signature, "sha256="+tt.payloadSha)

			actual, err := ParseIncomingWebhook(
				context.Background(),
				vcsclient.EmptyLogger{},
				WebhookOrigin{
					VcsProvider: vcsutils.BitbucketServer,
					Token:       token,
				},
				request,
			)
			assert.NoError(t, err)
			assert.Equal(t, &WebhookInfo{Event: tt.expectedEventType, Tag: tt.expectedTagInfo}, actual)
		})
	}
}

func TestBitbucketServerParseIncomingWebhookError(t *testing.T) {
	request := &http.Request{Body: io.NopCloser(io.MultiReader())}
	_, err := ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.BitbucketServer,
			Token:       token,
		},
		request)
	assert.Error(t, err)

	webhook := bitbucketServerWebhookParser{}
	_, err = webhook.parseIncomingWebhook(context.Background(), request, []byte{})
	assert.Error(t, err)
}

func TestBitbucketServerParsePrEventsError(t *testing.T) {
	webhook := bitbucketServerWebhookParser{logger: vcsclient.EmptyLogger{}}
	_, err := webhook.parsePrEvents(&bitbucketServerWebHook{}, vcsutils.Push)
	assert.Error(t, err)
}

func TestBitbucketServerPayloadMismatchSignature(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "pushpayload.json"))
	assert.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest(http.MethodPost, "https://127.0.0.1", reader)
	request.Header.Add(EventHeaderKey, "repo:refs_changed")
	request.Header.Add(sha256Signature, "sha256=wrongsianature")

	// Parse webhook
	_, err = ParseIncomingWebhook(context.Background(),
		vcsclient.EmptyLogger{},
		WebhookOrigin{
			VcsProvider: vcsutils.BitbucketServer,
			Token:       token,
		},
		request)
	assert.EqualError(t, err, "payload signature mismatch")
}

func formatOwnerForBitbucketServer(owner string) string {
	return fmt.Sprintf("~%s", strings.ToUpper(owner))
}
