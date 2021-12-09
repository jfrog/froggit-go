package webhookparser

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	bitbucketServerExpectedPrId = 3
)

func TestBitbucketServerParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "pushpayload"))
	assert.NoError(t, err)
	defer closeReader(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(EventHeaderKey, "repo:refs_changed")
	request.Header.Add(Sha256Signature, "sha256="+bitbucketServerPushSha256)

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketServer, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, bitbucketServerPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
}

func TestBitbucketServerParseIncomingPrCreateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "prcreatepayload"))
	assert.NoError(t, err)
	defer closeReader(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1?", reader)
	request.Header.Add(EventHeaderKey, "pr:opened")
	request.Header.Add(Sha256Signature, "sha256="+bitbucketServerPrCreatedSha256)

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketServer, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, bitbucketServerExpectedPrId, actual.PullRequestId)
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, bitbucketServerPrCreateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepositoryDetails.Name)
	assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.SourceRepositoryDetails.Owner)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrCreated, actual.Event)
}

func TestBitbucketServerParseIncomingPrUpdateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "prupdatepayload"))
	assert.NoError(t, err)
	defer closeReader(reader)

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(EventHeaderKey, "pr:from_ref_updated")
	request.Header.Add(Sha256Signature, "sha256="+bitbucketServerPrUpdatedSha256)

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.BitbucketServer, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, bitbucketServerExpectedPrId, actual.PullRequestId)
	assert.Equal(t, expectedRepoName, actual.TargetRepositoryDetails.Name)
	assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.TargetRepositoryDetails.Owner)
	assert.Equal(t, expectedBranch, actual.TargetBranch)
	assert.Equal(t, bitbucketServerPrUpdateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepositoryDetails.Name)
	assert.Equal(t, formatOwnerForBitbucketServer(expectedOwner), actual.SourceRepositoryDetails.Owner)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrEdited, actual.Event)
}

func TestBitbucketServerPayloadMismatchSignature(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "bitbucketserver", "pushpayload"))
	assert.NoError(t, err)
	defer closeReader(reader)

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
