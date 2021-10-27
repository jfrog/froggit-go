package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/ktrysmt/go-bitbucket"
	"github.com/stretchr/testify/assert"
)

func TestBitbucketCloud_Connection(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucket.User{"values": {}}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/user", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestBitbucketCloud_ConnectionWhenContextCancelled(t *testing.T) {
	t.Skip("Bitbucket cloud does not use the context")
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.BitbucketCloud, 0)
	defer cleanUp()
	err := client.TestConnection(ctxWithCancel)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBitbucketCloud_ConnectionWhenContextTimesOut(t *testing.T) {
	t.Skip("Bitbucket cloud does not use the context")
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.BitbucketCloud, 50*time.Millisecond)
	defer cleanUp()
	err := client.TestConnection(ctxWithTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestBitbucketCloud_ListRepositories(t *testing.T) {
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

func TestBitbucketCloud_ListBranches(t *testing.T) {
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

func TestBitbucketCloud_CreateWebhook(t *testing.T) {
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

func TestBitbucketCloud_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)
	defer cleanUp()

	err = client.UpdateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", token, id.String(),
		vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestBitbucketCloud_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)

	defer cleanUp()

	err = client.DeleteWebhook(ctx, owner, repo1, id.String())
	assert.NoError(t, err)
}

func TestBitbucketCloud_SetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/commit/%s/statuses/build", ref), createBitbucketCloudHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Pass, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestBitbucketCloud_DownloadRepository(t *testing.T) {
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

func TestBitbucketCloud_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, "/repositories/jfrog/repo-1/pullrequests/", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func TestBitbucketCloud_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commits/%s?pagelen=1", owner, repo1, "master"), createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	require.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "ec05bacb91d757b4b6b2a11a0676471020e89fb5",
		AuthorName:    "user",
		CommitterName: "",
		Url:           "https://api.bitbucket.org/2.0/repositories/user2/setup-jfrog-cli/commit/ec05bacb91d757b4b6b2a11a0676471020e89fb5",
		Timestamp:     1591040823,
		Message:       "Fix README.md: yaml\n",
		ParentHashes:  []string{"774aa0fb252bccbc2a7e01060ef4d4be0b0eeaa9", "def26c6128ebe11fac555fe58b59227e9655dc4d"},
	}, result)
}

func TestBitbucketCloud_GetLatestCommitNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`<!DOCTYPE html><html lang="en"></html>`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commits/%s?pagelen=1", owner, repo1, "master"), http.StatusNotFound,
		createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")
	require.EqualError(t, err, "404 Not Found")
	assert.Empty(t, result)
}

func TestBitbucketCloud_GetLatestCommitInvalidPayload(t *testing.T) {
	tests := []struct {
		name   string
		owner  string
		repo   string
		branch string
	}{
		{
			name:   "all empty",
			owner:  "",
			repo:   "",
			branch: "",
		},
		{
			name:   "empty owner",
			owner:  "",
			repo:   "repo",
			branch: "branch",
		},
		{
			name:   "empty repo",
			owner:  "owner",
			repo:   "",
			branch: "branch",
		},
		{
			name:   "empty branch",
			owner:  "owner",
			repo:   "repo",
			branch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
			require.NoError(t, err)

			result, err := client.GetLatestCommit(ctx, tt.owner, tt.repo, tt.branch)

			require.EqualError(t, err, "required parameter is empty")
			assert.Empty(t, result)
		})
	}
}

func createBitbucketCloudHandler(t *testing.T, expectedUri string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
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
