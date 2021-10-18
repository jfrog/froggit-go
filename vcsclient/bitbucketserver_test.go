package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

func TestBitbucketServer_Connection(t *testing.T) {
	ctx := context.Background()
	mockResponse := make(map[string][]bitbucketv1.User)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, mockResponse,
		"/api/1.0/admin/users?limit=1", createBitbucketServerHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestBitbucketServer_ConnectionWhenContextCancelled(t *testing.T) {
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.BitbucketServer, 0)
	defer cleanUp()
	err := client.TestConnection(ctxWithCancel)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBitbucketServer_ConnectionWhenContextTimesOut(t *testing.T) {
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.BitbucketServer, 50*time.Millisecond)
	defer cleanUp()
	err := client.TestConnection(ctxWithTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestBitbucketServer_ListRepositories(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, "", createBitbucketServerListRepositoriesHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"~" + username: {repo1}, username: {repo2}}, actualRepositories)
}

func TestBitbucketServer_ListBranches(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucketv1.Branch{
		"values": {{ID: branch1}, {ID: branch2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, mockResponse, "/api/1.0/projects/jfrog/repos/repo-1/branches", createBitbucketServerHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})
}

func TestBitbucketServer_CreateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31()
	mockResponse := bitbucketv1.Webhook{ID: int(id)}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, mockResponse, "/api/1.0/projects/jfrog/repos/repo-1/webhooks", createBitbucketServerHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything",
		vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, strconv.Itoa(int(id)), actualId)
}

func TestBitbucketServer_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31()
	stringId := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringId), createBitbucketServerHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", token, stringId,
		vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestBitbucketServer_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31()
	stringId := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringId), createBitbucketServerHandler)
	defer cleanUp()

	err := client.DeleteWebhook(ctx, owner, repo1, stringId)
	assert.NoError(t, err)
}

func TestBitbucketServer_SetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/build-status/1.0/commits/%s", ref), createBitbucketServerHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Fail, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestBitbucketServer_DownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	client, err := NewClientBuilder(vcsutils.BitbucketServer).ApiEndpoint("https://open-bitbucket.nrao.edu/rest").Build()
	assert.NoError(t, err)

	err = client.DownloadRepository(ctx, "ssa", "solr-system", "master", dir)
	assert.NoError(t, err)

	_, err = os.OpenFile(filepath.Join(dir, "README.md"), os.O_RDONLY, 0644)
	assert.NoError(t, err)
}

func TestBitbucketServer_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, "/api/1.0/projects/jfrog/repos/repo-1/pull-requests", createBitbucketServerHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func TestBitbucketServer_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "commit_list_response.json"))
	assert.NoError(t, err)

	// limit=1 appears twice because it is added twice by: github.com/gfleury/go-bitbucket-v1@v0.0.0-20210826163055-dff2223adeac/default_api.go:3848
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, response,
		fmt.Sprintf("/api/1.0/projects/%s/repos/%s/commits?limit=1&limit=1&until=master", owner, repo1),
		createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	require.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "def0123abcdef4567abcdef8987abcdef6543abc",
		AuthorName:    "charlie",
		CommitterName: "mark",
		Url:           "",
		Timestamp:     1548720847610,
		Message:       "More work on feature 1",
		ParentHashes:  []string{"abcdef0123abcdef4567abcdef8987abcdef6543", "qwerty0123abcdef4567abcdef8987abcdef6543"},
	}, result)
}

func TestBitbucketServer_GetLatestCommitNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
		"errors": [
        	{
            	"context": null,
            	"exceptionName": "com.atlassian.bitbucket.project.NoSuchProjectException",
            	"message": "Project unknown does not exist."
        	}
    	]
	}`)
	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketServer, false, response,
		fmt.Sprintf("/api/1.0/projects/%s/repos/%s/commits?limit=1&limit=1&until=master", owner, repo1),
		http.StatusNotFound, createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Status: 404 Not Found")
	assert.Empty(t, result)
}

func TestBitbucketServer_GetLatestCommitInvalidPayload(t *testing.T) {
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
			client, err := NewClientBuilder(vcsutils.BitbucketServer).Build()
			require.NoError(t, err)

			result, err := client.GetLatestCommit(ctx, tt.owner, tt.repo, tt.branch)

			require.EqualError(t, err, "required parameter is empty")
			assert.Empty(t, result)
		})
	}
}

func createBitbucketServerHandler(t *testing.T, expectedUri string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		require.NoError(t, err)
		assert.Equal(t, expectedUri, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	}
}

func createBitbucketServerListRepositoriesHandler(t *testing.T, _ string, _ []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var responseObj interface{}
		if r.RequestURI == "/api/1.0/projects?start=0" {
			responseObj = map[string][]bitbucketv1.Project{"values": {{Key: username}}}
			w.Header().Add("X-Ausername", username)

		} else if r.RequestURI == "/api/1.0/projects/~frogger/repos?start=0" {
			responseObj = map[string][]bitbucketv1.Repository{"values": {{Slug: repo1}}}
		} else if r.RequestURI == "/api/1.0/projects/frogger/repos?start=0" {
			responseObj = map[string][]bitbucketv1.Repository{"values": {{Slug: repo2}}}
		} else {
			assert.Fail(t, "Unexpected request Uri "+r.RequestURI)
		}
		w.WriteHeader(expectedStatusCode)
		response, err := json.Marshal(responseObj)
		require.NoError(t, err)
		_, err = w.Write(response)
		require.NoError(t, err)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	}
}
