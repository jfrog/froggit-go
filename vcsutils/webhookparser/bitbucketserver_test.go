package webhookparser

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
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

	bitbucketServerExpectedPrId = 3
)

func TestBitbucketServerParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "pushpayload.json"))
	require.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(EventHeaderKey, "repo:refs_changed")
	request.Header.Add(Sha256Signature, "sha256="+bitbucketServerPushSha256)

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketServer, token, request)
	require.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, bitbucketServerPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
}

func TestBitbucketServerParseIncomingPrWebhook(t *testing.T) {
	tests := []struct {
		name              string
		payloadFilename   string
		eventHeader       string
		payloadSha        string
		expectedTime      int64
		expectedEventType vcsutils.WebhookEvent
	}{
		{
			name:              "create",
			payloadFilename:   "prcreatepayload.json",
			eventHeader:       "pr:opened",
			payloadSha:        bitbucketServerPrCreatedSha256,
			expectedTime:      bitbucketServerPrCreateExpectedTime,
			expectedEventType: vcsutils.PrOpened,
		},
		{
			name:              "update",
			payloadFilename:   "prupdatepayload.json",
			eventHeader:       "pr:from_ref_updated",
			payloadSha:        bitbucketServerPrUpdatedSha256,
			expectedTime:      bitbucketServerPrUpdateExpectedTime,
			expectedEventType: vcsutils.PrEdited,
		},
		{
			name:              "merge",
			payloadFilename:   "prmergepayload.json",
			eventHeader:       "pr:merged",
			payloadSha:        bitbucketServerPrMergedSha256,
			expectedTime:      bitbucketServerPrMergeExpectedTime,
			expectedEventType: vcsutils.PrMerged,
		},
		{
			name:              "decline",
			payloadFilename:   "prdeclinepayload.json",
			eventHeader:       "pr:declined",
			payloadSha:        bitbucketServerPrDeclinedSha256,
			expectedTime:      bitbucketServerPrDeclineExpectedTime,
			expectedEventType: vcsutils.PrRejected,
		},
		{
			name:              "delete",
			payloadFilename:   "prdeletepayload.json",
			eventHeader:       "pr:deleted",
			payloadSha:        bitbucketServerPrDeletedSha256,
			expectedTime:      bitbucketServerPrDeleteExpectedTime,
			expectedEventType: vcsutils.PrRejected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", tt.payloadFilename))
			require.NoError(t, err)
			defer close(reader)

			// Create request
			request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
			request.Header.Add(EventHeaderKey, tt.eventHeader)
			request.Header.Add(Sha256Signature, "sha256="+tt.payloadSha)

			// Parse webhook
			actual, err := ParseIncomingWebhook(vcsutils.BitbucketServer, token, request)
			require.NoError(t, err)

			// Check values
			assert.Equal(t, bitbucketServerExpectedPrId, actual.PullRequestId)
			assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
			assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.TargetRepositoryDetails.Owner)
			assert.Equal(t, expectedBranch, actual.TargetBranch)
			assert.Equal(t, tt.expectedTime, actual.Timestamp)
			assert.Equal(t, expectedRepoName, actual.SourceRepositoryDetails.Name)
			assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.SourceRepositoryDetails.Owner)
			assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
			assert.Equal(t, tt.expectedEventType, actual.Event)
		})
	}
}

func TestBitbucketServerParseIncomingWebhookError(t *testing.T) {
	_, err := ParseIncomingWebhook(vcsutils.BitbucketServer, token, &http.Request{Body: io.NopCloser(io.MultiReader())})
	require.Error(t, err)

	webhook := BitbucketServerWebhook{}
	_, err = webhook.parseIncomingWebhook([]byte{})
	assert.Error(t, err)
}

func TestBitbucketServerParsePrEventsError(t *testing.T) {
	webhook := BitbucketServerWebhook{request: &http.Request{}}
	_, err := webhook.parsePrEvents(&bitbucketServerWebHook{}, vcsutils.Push)
	assert.Error(t, err)
}

func TestBitbucketServerPayloadMismatchSignature(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "pushpayload.json"))
	require.NoError(t, err)
	defer close(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(EventHeaderKey, "repo:refs_changed")
	request.Header.Add(Sha256Signature, "sha256=wrongsianature")

	// Parse webhook
	_, err = ParseIncomingWebhook(vcsutils.BitbucketServer, token, request)
	assert.EqualError(t, err, "Payload signature mismatch")
}

func formatOwnerForBitbucketServer(owner string) string {
	return fmt.Sprintf("~%s", strings.ToUpper(owner))
}
