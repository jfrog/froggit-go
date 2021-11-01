package vcsclient

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v38/github"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

func TestGitHubClient_Connection(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, "It's Not Easy Being Green", "/zen", createGitHubHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestGitHubClient_ConnectionWhenContextCancelled(t *testing.T) {
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.GitHub, 0)
	defer cleanUp()
	err := client.TestConnection(ctxWithCancel)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGitHubClient_ConnectionWhenContextTimesOut(t *testing.T) {
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.GitHub, 50*time.Millisecond)
	defer cleanUp()
	err := client.TestConnection(ctxWithTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGitHubClient_ListRepositories(t *testing.T) {
	ctx := context.Background()
	expectedRepo1 := github.Repository{Name: &repo1, Owner: &github.User{Login: &username}}
	expectedRepo2 := github.Repository{Name: &repo2, Owner: &github.User{Login: &username}}
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []github.Repository{expectedRepo1, expectedRepo2}, "/user/repos", createGitHubHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, actualRepositories, map[string][]string{username: {repo1, repo2}})
}

func TestGitHubClient_ListBranches(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []github.Branch{{Name: &branch1}, {Name: &branch2}}, fmt.Sprintf("/repos/jfrog/%s/branches", repo1), createGitHubHandler)
	defer cleanUp()

	actualBranches, err := client.ListBranches(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualBranches, []string{branch1, branch2})
}

func TestGitHubClient_CreateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks", repo1), createGitHubHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, actualId, strconv.FormatInt(id, 10))
}

func TestGitHubClient_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks/%s", repo1, strconv.FormatInt(id, 10)), createGitHubHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", token, strconv.FormatInt(id, 10),
		vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestGitHubClient_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks/%s", repo1, strconv.FormatInt(id, 10)), createGitHubHandler)
	defer cleanUp()

	err := client.DeleteWebhook(ctx, owner, repo1, strconv.FormatInt(id, 10))
	assert.NoError(t, err)
}

func TestGitHubClient_CreateCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "39e5418"
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.RepoStatus{}, fmt.Sprintf("/repos/jfrog/%s/statuses/%s", repo1, ref), createGitHubHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Error, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestGitHubClient_DownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitHub, false,
		[]byte("https://github.com/octocat/Hello-World/archive/refs/heads/master.tar.gz"),
		"/repos/jfrog/Hello-World/tarball/test", http.StatusFound, createGitHubHandler)
	defer cleanUp()
	assert.NoError(t, err)

	err = client.DownloadRepository(ctx, owner, "Hello-World", "test", dir)
	require.NoError(t, err)
	fileinfo, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, fileinfo, 1)
	assert.Equal(t, "README", fileinfo[0].Name())
}

func TestGitHubClient_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.PullRequest{}, "/repos/jfrog/repo-1/pulls", createGitHubHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func TestGitHubClient_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=1&sha=master", owner, repo1), createGitHubHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	require.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "6dcb09b5b57875f334f61aebed695e2e4193db5e",
		AuthorName:    "Monalisa Octocat",
		CommitterName: "Joconde Octocat",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"6dcb09b5b57875f334f61aebed695e2e4193db5e"},
	}, result)
}

func TestGitHubClient_AddSshKeyToRepository(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
	 "id": 1,
	 "key": "ssh-rsa AAAA...",
	 "url": "https://api.github.com/repos/octocat/Hello-World/keys/1",
	 "title": "My deploy key",
	 "verified": true,
	 "created_at": "2014-12-10T15:53:42Z",
	 "read_only": true
	}`)

	expectedBody := []byte(`{"key":"ssh-rsa AAAA...","title":"My deploy key","read_only":true}` + "\n")

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.GitHub, false,
		response, fmt.Sprintf("/repos/%v/%v/keys", owner, repo1), http.StatusCreated, expectedBody, http.MethodPost,
		createGitHubWithBodyHandler)
	defer closeServer()

	err := client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", Read)

	require.NoError(t, err)
}

func TestGitHubClient_AddSshKeyToRepositoryReadWrite(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
	 "id": 1,
	 "key": "ssh-rsa AAAA...",
	 "url": "https://api.github.com/repos/octocat/Hello-World/keys/1",
	 "title": "My deploy key",
	 "verified": true,
	 "created_at": "2014-12-10T15:53:42Z",
	 "read_only": true
	}`)

	expectedBody := []byte(`{"key":"ssh-rsa AAAA...","title":"My deploy key","read_only":false}` + "\n")

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.GitHub, false,
		response, fmt.Sprintf("/repos/%v/%v/keys", owner, repo1), http.StatusCreated, expectedBody, http.MethodPost,
		createGitHubWithBodyHandler)
	defer closeServer()

	err := client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", ReadWrite)

	require.NoError(t, err)
}

func createGitHubWithBodyHandler(t *testing.T, expectedUri string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHttpMethod string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, expectedHttpMethod, request.Method)
		assert.Equal(t, expectedUri, request.RequestURI)
		assert.Equal(t, "Bearer "+token, request.Header.Get("Authorization"))

		b, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Equal(t, expectedRequestBody, b)

		writer.WriteHeader(expectedStatusCode)
		_, err = writer.Write(response)
		assert.NoError(t, err)
	}
}

func TestGitHubClient_GetRepositoryInfo(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "repository_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response, "/repos/octocat/Hello-World", createGitHubHandler)
	defer cleanUp()

	info, err := client.GetRepositoryInfo(ctx, "octocat", "Hello-World")
	require.NoError(t, err)
	require.Equal(t,
		RepositoryInfo{
			CloneInfo: CloneInfo{HTTP: "https://github.com/octocat/Hello-World.git", SSH: "git@github.com:octocat/Hello-World.git"},
		},
		info,
	)
}

func createGitHubHandler(t *testing.T, expectedUri string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedUri, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		if strings.Contains(r.RequestURI, "tarball") {
			w.Header().Add("Location", string(response))
			w.WriteHeader(expectedStatusCode)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		require.NoError(t, err)
	}
}
