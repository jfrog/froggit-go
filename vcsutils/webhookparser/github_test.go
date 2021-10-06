package webhookparser

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

const (
	githubSha256Header = "X-HUB-SIGNATURE-256"
	githubEventHeader  = "X-GITHUB-EVENT"

	// Push event
	githubPushSha256       = "687737b6d39345e557be42058da1ad57dbd5f54baeb30044751e50d396cc2116"
	githubPushExpectedTime = int64(1630416256)
	// Pull request create event
	githubPrCreateSha256       = "48b9f23bfeb95dd8a1067b590f753599dbd12732c8d5217431ec70132cee8c1c"
	githubPrCreateExpectedTime = int64(1630666350)
	// Pull request update event
	githubPrUpdateSha256       = "4e03348ae316bae2719c0e936b55535e7869a85aa5649021ea5055b378cd6d56"
	githubPrUpdateExpectedTime = int64(1630666481)
)

func TestGitHubParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "github", "pushpayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add("content-type", "application/x-www-form-urlencoded")
	request.Header.Add(githubSha256Header, "sha256="+githubPushSha256)
	request.Header.Add(githubEventHeader, "push")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.GitHub, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, githubPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
}

func TestGitHubParseIncomingPrCreateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "github", "prcreatepayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add("content-type", "application/x-www-form-urlencoded")
	request.Header.Add(githubSha256Header, "sha256="+githubPrCreateSha256)
	request.Header.Add(githubEventHeader, "pull_request")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.GitHub, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, githubPrCreateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepository)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrCreated, actual.Event)
}

func TestGitHubParseIncomingPrUpdateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "github", "prupdatepayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add("content-type", "application/x-www-form-urlencoded")
	request.Header.Add(githubSha256Header, "sha256="+githubPrUpdateSha256)
	request.Header.Add(githubEventHeader, "pull_request")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.GitHub, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, githubPrUpdateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepository)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrEdited, actual.Event)
}

func TestGitHubPayloadMismatchSignature(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "github", "pushpayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add("content-type", "application/x-www-form-urlencoded")
	request.Header.Add(githubSha256Header, "sha256=wrongsignature")
	request.Header.Add(githubEventHeader, "push")

	// Parse webhook
	_, err = ParseIncomingWebhook(vcsutils.GitHub, token, request)
	assert.True(t, strings.HasPrefix(err.Error(), "error decoding signature"), "error was: "+err.Error())
}
