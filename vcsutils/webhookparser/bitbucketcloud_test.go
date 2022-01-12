package webhookparser

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
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
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketCloud, token, request)
	require.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, expectedOwner, actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, bitbucketCloudPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
}

func TestBitbucketCloudParseIncomingPrWebhook(t *testing.T) {
	tests := []struct {
		name              string
		payloadFilename   string
		eventHeader       string
		expectedTime      int64
		expectedEventType vcsutils.WebhookEvent
	}{
		{
			name:              "create",
			payloadFilename:   "prcreatepayload.json",
			eventHeader:       "pullrequest:created",
			expectedTime:      bitbucketCloudPrCreateExpectedTime,
			expectedEventType: vcsutils.PrOpened,
		},
		{
			name:              "update",
			payloadFilename:   "prupdatepayload.json",
			eventHeader:       "pullrequest:updated",
			expectedTime:      bitbucketCloudPrUpdateExpectedTime,
			expectedEventType: vcsutils.PrEdited,
		},
		{
			name:              "merge",
			payloadFilename:   "prmergepayload.json",
			eventHeader:       "pullrequest:fulfilled",
			expectedTime:      bitbucketCloudPrMergeExpectedTime,
			expectedEventType: vcsutils.PrMerged,
		},
		{
			name:              "close",
			payloadFilename:   "prclosepayload.json",
			eventHeader:       "pullrequest:rejected",
			expectedTime:      bitbucketCloudPrCloseExpectedTime,
			expectedEventType: vcsutils.PrRejected,
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
			actual, err := ParseIncomingWebhook(vcsutils.BitbucketCloud, token, request)
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
		})
	}
}

func TestBitbucketCloudParseIncomingWebhookError(t *testing.T) {
	_, err := ParseIncomingWebhook(vcsutils.BitbucketCloud, token, &http.Request{URL: &url.URL{RawQuery: "token=a"}})
	assert.Error(t, err)

	webhook := BitbucketCloudWebhook{}
	_, err = webhook.parseIncomingWebhook([]byte{})
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
	_, err = ParseIncomingWebhook(vcsutils.BitbucketCloud, token, request)
	assert.EqualError(t, err, "token mismatch")
}
