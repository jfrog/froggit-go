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

func TestBitbucketServerConnection(t *testing.T) {
	ctx := context.Background()
	mockResponse := make(map[string][]bitbucketv1.User)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, mockResponse, "/api/1.0/admin/users", createBitbucketServerHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestBitbucketConnectionWhenContextCancelled(t *testing.T) {
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel()

	client, closeServer := createWaitingServerAndClient(t, vcsutils.BitbucketServer, 0)
	defer closeServer()
	err := client.TestConnection(ctxWithCancel)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBitbucketConnectionWhenContextTimesOut(t *testing.T) {
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	client, closeServer := createWaitingServerAndClient(t, vcsutils.BitbucketServer, 50*time.Millisecond)
	defer closeServer()
	err := client.TestConnection(ctxWithTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestBitbucketServerListRepositories(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, "", createBitbucketServerListRepositoriesHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"~" + username: {repo1}, username: {repo2}}, actualRepositories)
}

func TestBitbucketServerListBranches(t *testing.T) {
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

func TestBitbucketServerCreateWebhook(t *testing.T) {
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

func TestBitbucketServerUpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31()
	stringId := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringId), createBitbucketServerHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", token, stringId,
		vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestBitbucketServerDeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31()
	stringId := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringId), createBitbucketServerHandler)
	defer cleanUp()

	err := client.DeleteWebhook(ctx, owner, repo1, stringId)
	assert.NoError(t, err)
}

func TestBitbucketServerSetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/build-status/1.0/commits/%s", ref), createBitbucketServerHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Fail, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestBitbucketServerDownloadRepository(t *testing.T) {
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

func TestBitbucketServerCreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, "/api/1.0/projects/jfrog/repos/repo-1/pull-requests", createBitbucketServerHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func createBitbucketServerHandler(t *testing.T, expectedUri string, response []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(response)
		require.NoError(t, err)
		assert.Equal(t, expectedUri, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	}
}

func createBitbucketServerListRepositoriesHandler(t *testing.T, _ string, _ []byte) http.HandlerFunc {
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
		w.WriteHeader(http.StatusOK)
		response, err := json.Marshal(responseObj)
		require.NoError(t, err)
		_, err = w.Write(response)
		require.NoError(t, err)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	}
}
