package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/ktrysmt/go-bitbucket"
	"github.com/stretchr/testify/assert"
)

func TestBitbucketCloudConnection(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucket.User{"values": {}}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/user", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestBitbucketCloudConnectionWhenContextCancelled(t *testing.T) {
	t.Skip("Bitbucket cloud does not use the context")
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel()

	client, closeServer := createWaitingServerAndClient(t, vcsutils.BitbucketCloud, 0)
	defer closeServer()
	err := client.TestConnection(ctxWithCancel)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBitbucketCloudConnectionWhenContextTimesOut(t *testing.T) {
	t.Skip("Bitbucket cloud does not use the context")
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	client, closeServer := createWaitingServerAndClient(t, vcsutils.BitbucketCloud, 50*time.Millisecond)
	defer closeServer()
	err := client.TestConnection(ctxWithTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestBitbucketCloudListRepositories(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucket.Repository{
		"values": {{Slug: repo1}, {Slug: repo2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/repositories/"+username, createBitbucketCloudHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{username: {repo1, repo2}}, actualRepositories)
}

func TestBitbucketCloudListBranches(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucket.BranchModel{
		"values": {{Name: branch1}, {Name: branch2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/repositories/jfrog/repo-1/refs/branches?", createBitbucketCloudHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})
}

func TestBitbucketCloudCreateWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	mockResponse := bitbucket.WebhooksOptions{Uuid: "{" + id.String() + "}"}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/repositories/jfrog/repo-1/hooks", createBitbucketCloudHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything",
		vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, id.String(), actualId)
}

func TestBitbucketCloudUpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)
	defer cleanUp()

	err = client.UpdateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", token, id.String(),
		vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestBitbucketCloudDeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)

	defer cleanUp()

	err = client.DeleteWebhook(ctx, owner, repo1, id.String())
	assert.NoError(t, err)
}

func TestBitbucketCloudSetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/commit/%s/statuses/build", ref), createBitbucketCloudHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Pass, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestBitbucketCloudDownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.DownloadRepository(ctx, owner, "jfrog-setup-cli", "master", dir)
	assert.NoError(t, err)
	rootFiles, err := ioutil.ReadDir(dir)
	assert.NotEmpty(t, rootFiles)
	readmeFound := false
	for _, file := range rootFiles {
		if file.Name() == "README.md" {
			readmeFound = true
		}
	}
	assert.True(t, readmeFound)
}

func TestBitbucketCloudCreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, "/repositories/jfrog/repo-1/pullrequests/", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func createBitbucketCloudHandler(t *testing.T, expectedUri string, response []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.RequestURI == "/workspaces" {
			workspacesResults := make(map[string]interface{})
			workspacesResults["values"] = []bitbucket.Workspace{{Slug: username}}
			response, err := json.Marshal(workspacesResults)
			require.NoError(t, err)
			_, err = w.Write(response)
			require.NoError(t, err)
		} else {
			_, err := w.Write(response)
			require.NoError(t, err)
			assert.Equal(t, expectedUri, r.RequestURI)
		}
		assert.Equal(t, basicAuthHeader, r.Header.Get("Authorization"))
	}
}
