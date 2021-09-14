package webhookparser

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

const (
	bitbucketCloudPushExpectedTime     = int64(1630824565)
	bitbucketCloudPrCreateExpectedTime = int64(1630831665)
	bitbucketCloudPrUpdateExpectedTime = int64(1630844170)
)

func TestBitbucketCloudParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketcloud", "pushpayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1?token="+string(token), reader)
	request.Header.Add(EventHeaderKey, "repo:push")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketCloud, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, bitbucketCloudPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
}

func TestBitbucketCloudParseIncomingPrCreateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketcloud", "prcreatepayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1?token="+string(token), reader)
	request.Header.Add(EventHeaderKey, "pullrequest:created")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketCloud, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, bitbucketCloudPrCreateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepository)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrCreated, actual.Event)
}

func TestBitbucketCloudParseIncomingPrUpdateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketcloud", "prupdatepayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1?token="+string(token), reader)
	request.Header.Add(EventHeaderKey, "pullrequest:updated")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketCloud, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, bitbucketCloudPrUpdateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepository)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrEdited, actual.Event)
}

func TestBitbucketCloudPayloadMismatchToken(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketcloud", "pushpayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1?token=wrong-token", reader)
	request.Header.Add(EventHeaderKey, "repo:push")

	// Parse webhook
	_, err = ParseIncomingWebhook(vcsutils.BitbucketCloud, token, request)
	assert.EqualError(t, err, "Token mismatch")
}
