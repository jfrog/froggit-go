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
	gitLabEventHeader          = "X-GitLab-Event"
	gitlabPushExpectedTime     = int64(1630306883)
	gitlabPrCreateExpectedTime = int64(1631202047)
	gitlabPrUpdateExpectedTime = int64(1631202266)
)

func TestGitLabParseIncomingPushWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "pushpayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, string(token))
	request.Header.Add(gitLabEventHeader, "Push Hook")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.GitLab, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, gitlabPushExpectedTime, actual.Timestamp)
	assert.Equal(t, vcsutils.Push, actual.Event)
}

func TestGitLabParseIncomingPrCreateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "prcreatepayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, string(token))
	request.Header.Add(gitLabEventHeader, "Merge Request Hook")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.GitLab, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, gitlabPrCreateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepository)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrCreated, actual.Event)
}

func TestGitLabParseIncomingPrUpdateWebhook(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "prupdatepayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, string(token))
	request.Header.Add(gitLabEventHeader, "Merge Request Hook")

	// Parse webhook
	actual, err := ParseIncomingWebhook(vcsutils.GitLab, token, request)
	assert.NoError(t, err)

	// Check values
	assert.Equal(t, expectedRepoName, actual.Repository)
	assert.Equal(t, expectedBranch, actual.Branch)
	assert.Equal(t, gitlabPrUpdateExpectedTime, actual.Timestamp)
	assert.Equal(t, expectedRepoName, actual.SourceRepository)
	assert.Equal(t, expectedSourceBranch, actual.SourceBranch)
	assert.Equal(t, vcsutils.PrEdited, actual.Event)
}

func TestGitLabPayloadMismatchSignature(t *testing.T) {
	reader, err := os.Open(filepath.Join("testdata", "gitlab", "pushpayload"))
	assert.NoError(t, err)
	defer reader.Close()

	// Create request
	request := httptest.NewRequest("POST", "https://127.0.0.1", reader)
	request.Header.Add(gitLabKeyHeader, "wrong-token")
	request.Header.Add(gitLabEventHeader, "Push Hook")

	// Parse webhook
	_, err = ParseIncomingWebhook(vcsutils.GitLab, token, request)
	assert.EqualError(t, err, "Token mismatch")
}
