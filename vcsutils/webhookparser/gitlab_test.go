package webhookparser

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xanzy/go-gitlab"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

const (
	gitLabEventHeader          = "X-GitLab-Event"
	gitlabPushExpectedTime     = int64(1630306883)
	gitlabPrOpenExpectedTime   = int64(1631202047)
	gitlabPrReopenExpectedTime = int64(1638865856)
	gitlabPrUpdateExpectedTime = int64(1631202266)
	gitlabPrCloseExpectedTime  = int64(1638864453)
	gitlabPrMergeExpectedTime  = int64(1638866119)
	gitlabExpectedPrId         = 1
)

func TestGitLabParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "pushpayload.json"))
	require.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, string(token))
	request.Header.Add(gitLabEventHeader, "Push Hook")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.GitLab, token, request)
	require.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, expectedOwner, actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, gitlabPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
}

func TestGitLabParseIncomingPrWebhook(t *testing.T) {
	tests := []struct {
		name              string
		payloadFilename   string
		expectedTime      int64
		expectedEventType vcsutils.WebhookEvent
	}{
		{
			name:              "open",
			payloadFilename:   "propenpayload.json",
			expectedTime:      gitlabPrOpenExpectedTime,
			expectedEventType: vcsutils.PrOpened,
		},
		{
			name:              "reopen",
			payloadFilename:   "prreopenpayload.json",
			expectedTime:      gitlabPrReopenExpectedTime,
			expectedEventType: vcsutils.PrOpened,
		},
		{
			name:              "update",
			payloadFilename:   "prupdatepayload.json",
			expectedTime:      gitlabPrUpdateExpectedTime,
			expectedEventType: vcsutils.PrEdited,
		},
		{
			name:              "close",
			payloadFilename:   "prclosepayload.json",
			expectedTime:      gitlabPrCloseExpectedTime,
			expectedEventType: vcsutils.PrRejected,
		},
		{
			name:              "merge",
			payloadFilename:   "prmergepayload.json",
			expectedTime:      gitlabPrMergeExpectedTime,
			expectedEventType: vcsutils.PrMerged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := os.Open(filepath.Join("testdata", "gitlab", tt.payloadFilename))
			require.NoError(t, err)
			defer close(reader)

			// Create request
			request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
			request.Header.Add(gitLabKeyHeader, string(token))
			request.Header.Add(gitLabEventHeader, "Merge Request Hook")

			// Parse webhook
			actual, err := ParseIncomingWebhook(vcsutils.GitLab, token, request)
			require.NoError(t, err)

			// Check values
			assert.Equal(t, gitlabExpectedPrId, actual.PullRequestId)
			assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
			assert.Equal(t, expectedOwner, actual.TargetRepositoryDetails.Owner)
			assert.Equal(t, expectedBranch, actual.TargetBranch)
			assert.Equal(t, tt.expectedTime, actual.Timestamp)
			assert.Equal(t, expectedRepoName, actual.SourceRepositoryDetails.Name)
			assert.Equal(t, expectedOwner, actual.SourceRepositoryDetails.Owner)
			assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
			assert.Equal(t, tt.expectedEventType, actual.Event)
		})
	}
}

func TestGitLabParseIncomingWebhookError(t *testing.T) {
	_, err := ParseIncomingWebhook(vcsutils.GitLab, token, &http.Request{})
	require.Error(t, err)

	webhook := GitLabWebhook{request: &http.Request{}}
	_, err = webhook.parseIncomingWebhook([]byte{})
	assert.Error(t, err)
}

func TestGitLabParsePrEventsError(t *testing.T) {
	webhook := GitLabWebhook{}
	webhookInfo, _ := webhook.parsePrEvents(&gitlab.MergeEvent{})
	assert.Nil(t, webhookInfo)
}

func TestGitLabPayloadMismatchSignature(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "pushpayload.json"))
	require.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, "wrong-token")
	request.Header.Add(gitLabEventHeader, "Push Hook")

	// Parse webhook
	_, err = ParseIncomingWebhook(vcsutils.GitLab, token, request)
	assert.EqualError(t, err, "Token mismatch")
}
