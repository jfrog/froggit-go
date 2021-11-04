package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

func TestGitLabClient_Connection(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []gitlab.Project{}, "/api/v4/projects", createGitLabHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestGitLabClient_ConnectionWhenContextCancelled(t *testing.T) {
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.GitLab, 0)
	defer cleanUp()

	err := client.TestConnection(ctxWithCancel)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGitLabClient_ConnectionWhenContextTimesOut(t *testing.T) {
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.GitLab, 50*time.Millisecond)
	defer cleanUp()
	err := client.TestConnection(ctxWithTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGitLabClient_ListRepositories(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []gitlab.Project{{Path: repo1}, {Path: repo2}}, "/api/v4/groups/frogger/projects?page=1", createGitLabHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, actualRepositories, map[string][]string{username: {repo1, repo2}})
}

func TestGitLabClient_ListBranches(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []gitlab.Branch{{Name: branch1}, {Name: branch2}}, fmt.Sprintf("/api/v4/projects/%s/repository/branches", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})
}

func TestGitLabClient_CreateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", vcsutils.Push,
		vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, actualId, strconv.Itoa(id))
}

func TestGitLabClient_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks/%d", url.PathEscape(owner+"/"+repo1), id), createGitLabHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", token, strconv.Itoa(id),
		vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestGitLabClient_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks/%d", url.PathEscape(owner+"/"+repo1), id), createGitLabHandler)
	defer cleanUp()

	err := client.DeleteWebhook(ctx, owner, repo1, strconv.Itoa(id))
	assert.NoError(t, err)
}

func TestGitLabClient_CreateCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "5fbf81b31ff7a3b06bd362d1891e2f01bdb2be69"
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.CommitStatus{}, fmt.Sprintf("/api/v4/projects/%s/statuses/%s", url.PathEscape(owner+"/"+repo1), ref), createGitLabHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, InProgress, owner, repo1, ref, "Commit status title",
		"Commit status description", "https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestGitLabClient_DownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	repoFile, err := os.ReadFile(filepath.Join("testdata", "gitlab", "hello-world-main.tar.gz"))
	assert.NoError(t, err)

	ref := "5fbf81b31ff7a3b06bd362d1891e2f01bdb2be69"
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, repoFile, fmt.Sprintf("/api/v4/projects/%s/repository/archive.tar.gz?sha=%s", url.PathEscape(owner+"/"+repo1), ref), createGitLabHandler)
	defer cleanUp()

	err = client.DownloadRepository(ctx, owner, repo1, ref, dir)
	require.NoError(t, err)
	fileinfo, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, fileinfo, 1)
	assert.Equal(t, "README.md", fileinfo[0].Name())
}

func TestGitLabClient_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, &gitlab.MergeRequest{}, fmt.Sprintf("/api/v4/projects/%s/merge_requests", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func TestGitLabClient_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=1&ref_name=master",
			url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	require.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "ed899a2f4b50b4370feeea94676502b42383c746",
		AuthorName:    "Example User",
		CommitterName: "Administrator",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
		Timestamp:     1348131022,
		Message:       "Replace sanitize with escape once",
		ParentHashes:  []string{"6104942438c14ec7bd21c6cd5bd995272b3faff6"},
	}, result)
}

func TestGitLabClient_GetLatestCommitNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
    "message": "404 Project Not Found"
}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=1&ref_name=master",
			url.PathEscape(owner+"/"+repo1)), http.StatusNotFound, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404 Project Not Found")
	assert.Empty(t, result)
}

func TestGitLabClient_AddSshKeyToRepository(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
		"can_push": false,
		"created_at": "2021-10-21T13:49:59.996Z",
		"expires_at": null,
		"id": 1,
		"key": "ssh-rsa AAAA...",
		"title": "My deploy key"
	}`)
	expectedBody := []byte(`{"title":"My deploy key","key":"ssh-rsa AAAA...","can_push":false}`)

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/deploy_keys", url.PathEscape(owner+"/"+repo1)), http.StatusCreated,
		expectedBody, http.MethodPost, createGitLabWithBodyHandler)
	defer closeServer()

	err := client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", Read)

	require.NoError(t, err)
}

func TestGitLabClient_AddSshKeyToRepositoryReadWrite(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
		"can_push": false,
		"created_at": "2021-10-21T13:49:59.996Z",
		"expires_at": null,
		"id": 1,
		"key": "ssh-rsa AAAA...",
		"title": "My deploy key"
	}`)
	expectedBody := []byte(`{"title":"My deploy key","key":"ssh-rsa AAAA...","can_push":true}`)

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/deploy_keys", url.PathEscape(owner+"/"+repo1)), http.StatusCreated,
		expectedBody, http.MethodPost, createGitLabWithBodyHandler)
	defer closeServer()

	err := client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", ReadWrite)

	require.NoError(t, err)
}

func createGitLabHandler(t *testing.T, expectedUri string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/v4/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.RequestURI == "/api/v4/groups" {
			byteResponse, err := json.Marshal(&[]gitlab.Group{{Path: username}})
			assert.NoError(t, err)
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(byteResponse)
			assert.NoError(t, err)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Equal(t, expectedUri, r.RequestURI)
		assert.Equal(t, token, r.Header.Get("Private-Token"))
	}
}

func createGitLabWithBodyHandler(t *testing.T, expectedUri string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHttpMethod string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.RequestURI == "/api/v4/" {
			writer.WriteHeader(http.StatusOK)
			return
		}
		assert.Equal(t, expectedHttpMethod, request.Method)
		assert.Equal(t, expectedUri, request.RequestURI)
		assert.Equal(t, token, request.Header.Get("Private-Token"))

		b, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Equal(t, expectedRequestBody, b)

		writer.WriteHeader(expectedStatusCode)
		assert.NoError(t, err)
		_, err = writer.Write(response)
		assert.NoError(t, err)
	}
}
