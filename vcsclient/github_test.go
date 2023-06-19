package vcsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/google/go-github/v45/github"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

func TestGitHubClient_Connection(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, "It's Not Easy Being Green", "/zen", createGitHubHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)

	err = createBadGitHubClient(t).TestConnection(ctx)
	assert.Error(t, err)
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
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []github.Repository{expectedRepo1, expectedRepo2}, "/user/repos?page=1", createGitHubHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, actualRepositories, map[string][]string{username: {repo1, repo2}})

	_, err = createBadGitHubClient(t).ListRepositories(ctx)
	assert.Error(t, err)
}

func TestGitHubClient_ListRepositoriesWithPagination(t *testing.T) {
	ctx := context.Background()
	const repo = "repo"
	repos := make([]github.Repository, 0)
	repoNames := make([]string, 0)
	for i := 1; i <= 31; i++ {
		repoName := fmt.Sprintf("%v%v", repo, i)
		repos = append(repos, github.Repository{Name: &repoName, Owner: &github.User{Login: &username}})
		repoNames = append(repoNames, repoName)
	}

	client, cleanUp := createBodyHandlingServerAndClient(t, vcsutils.GitHub, false, repos, "/user/repos",
		http.StatusOK, nil, "GET", createGitHubWithPaginationHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(repos), len(actualRepositories[username]))
	assert.Equal(t, repoNames, actualRepositories[username])

	// Test Case 2 - No Items to return
	repos = make([]github.Repository, 0)
	client, cleanUp = createBodyHandlingServerAndClient(t, vcsutils.GitHub, false, repos, "/user/repos",
		http.StatusOK, nil, "GET", createGitHubWithPaginationHandler)
	defer cleanUp()

	actualRepositories, err = client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Nil(t, actualRepositories[username])

	_, err = createBadGitHubClient(t).ListRepositories(ctx)
	assert.Error(t, err)
}

func TestGitHubClient_ListBranches(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []github.Branch{{Name: &branch1}, {Name: &branch2}}, fmt.Sprintf("/repos/jfrog/%s/branches", repo1), createGitHubHandler)
	defer cleanUp()

	actualBranches, err := client.ListBranches(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualBranches, []string{branch1, branch2})

	_, err = createBadGitHubClient(t).ListBranches(ctx, owner, repo1)
	assert.Error(t, err)
}

func TestGitHubClient_CreateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks", repo1), createGitHubHandler)
	defer cleanUp()

	actualID, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, actualID, strconv.FormatInt(id, 10))

	_, _, err = createBadGitHubClient(t).CreateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", vcsutils.Push)
	assert.Error(t, err)
}

func TestGitHubClient_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks/%s", repo1, strconv.FormatInt(id, 10)), createGitHubHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", token, strconv.FormatInt(id, 10),
		vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected, vcsutils.TagPushed, vcsutils.TagRemoved)
	assert.NoError(t, err)

	err = createBadGitHubClient(t).UpdateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", token, strconv.FormatInt(id, 10),
		vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected)
	assert.Error(t, err)
}

func TestGitHubClient_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks/%s", repo1, strconv.FormatInt(id, 10)), createGitHubHandler)
	defer cleanUp()

	err := client.DeleteWebhook(ctx, owner, repo1, strconv.FormatInt(id, 10))
	assert.NoError(t, err)

	err = createBadGitHubClient(t).DeleteWebhook(ctx, "", "", "")
	assert.Error(t, err)
}

func TestGitHubClient_CreateCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "39e5418"
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.RepoStatus{}, fmt.Sprintf("/repos/jfrog/%s/statuses/%s", repo1, ref), createGitHubHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Error, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)

	err = createBadGitHubClient(t).SetCommitStatus(ctx, Error, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.Error(t, err)
}

func TestGitHubClient_getRepositoryVisibility(t *testing.T) {
	visibility := "public"
	assert.Equal(t, Public, getGitHubRepositoryVisibility(&github.Repository{Visibility: &visibility}))
	visibility = "internal"
	assert.Equal(t, Internal, getGitHubRepositoryVisibility(&github.Repository{Visibility: &visibility}))
	visibility = "private"
	assert.Equal(t, Private, getGitHubRepositoryVisibility(&github.Repository{Visibility: &visibility}))
}

func TestGitHubClient_getGitHubCommitState(t *testing.T) {
	assert.Equal(t, "success", getGitHubCommitState(Pass))
	assert.Equal(t, "failure", getGitHubCommitState(Fail))
	assert.Equal(t, "error", getGitHubCommitState(Error))
	assert.Equal(t, "pending", getGitHubCommitState(InProgress))
	assert.Equal(t, "", getGitHubCommitState(5))
}

func TestGitHubClient_DownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitHub, false,
		[]byte("https://github.com/octocat/Hello-World/archive/refs/heads/master.tar.gz"),
		"/repos/jfrog/Hello-World/tarball/test", http.StatusFound, createGitHubHandler)
	defer cleanUp()
	assert.NoError(t, err)

	err = client.DownloadRepository(ctx, owner, "Hello-World", "test", dir)
	require.NoError(t, err)
	fileinfo, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, fileinfo, 2)
	assert.Equal(t, ".git", fileinfo[0].Name())
	assert.Equal(t, "README", fileinfo[1].Name())

	err = createBadGitHubClient(t).DownloadRepository(ctx, owner, "Hello-World", "test", dir)
	assert.Error(t, err)
}

func TestGitHubClient_DownloadFileFromRepository(t *testing.T) {
	ctx := context.Background()
	downloadURL := "https://jfrog.com"
	name := "hello-world"
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, &[]github.RepositoryContent{{DownloadURL: &downloadURL, Name: &name}}, "/repos/jfrog/repo-1/contents/?ref=branch-1", createGitHubHandler)
	defer cleanUp()
	content, statusCode, err := client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "hello-world")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, content)

	_, _, err = client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "hello-bald")
	assert.Error(t, err)

	_, _, err = client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "hello-bald")
	assert.Error(t, err)

	_, _, err = createBadGitHubClient(t).DownloadFileFromRepo(ctx, owner, repo1, branch1, "hello")
	assert.Error(t, err)
}

func TestGitHubClient_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.PullRequest{}, "/repos/jfrog/repo-1/pulls", createGitHubHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)

	err = createBadGitHubClient(t).CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.Error(t, err)
}

func TestGitHubClient_AddPullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.IssueComment{}, "/repos/jfrog/repo-1/issues/1/comments", createGitHubHandler)
	defer cleanUp()

	err := client.AddPullRequestComment(ctx, owner, repo1, "Comment content", 1)
	assert.NoError(t, err)

	err = createBadGitHubClient(t).AddPullRequestComment(ctx, owner, repo1, "Comment content", 1)
	assert.Error(t, err)
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

	_, err = createBadGitHubClient(t).GetLatestCommit(ctx, owner, repo1, "master")
	assert.Error(t, err)
}

func TestGitHubClient_GetLatestCommitNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
    	"documentation_url": "https://docs.github.com/rest/reference/repos#list-commits",
    	"message": "Not Found"
	}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=1&sha=master", owner, "unknown"), http.StatusNotFound,
		createGitHubHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, "unknown", "master")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404 Not Found")
	assert.Empty(t, result)
}

func TestGitHubClient_GetLatestCommitUnknownBranch(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
    	"documentation_url": "https://docs.github.com/rest/reference/repos#list-commits",
    	"message": "Not Found"
	}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=1&sha=unknown", owner, repo1), http.StatusNotFound,
		createGitHubHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "unknown")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404 Not Found")
	assert.Empty(t, result)
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

	err = createBadGitHubClient(t).AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", Read)
	assert.Error(t, err)
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

func TestGitHubClient_GetCommitBySha(t *testing.T) {
	ctx := context.Background()
	sha := "6dcb09b5b57875f334f61aebed695e2e4193db5e"
	response, err := os.ReadFile(filepath.Join("testdata", "github", "commit_single_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits/%s", owner, repo1, sha), createGitHubHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)

	require.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          sha,
		AuthorName:    "Monalisa Octocat",
		CommitterName: "Joconde Octocat",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"5dcb09b5b57875f334f61aebed695e2e4193db5e"},
	}, result)

	_, err = createBadGitHubClient(t).GetCommitBySha(ctx, owner, repo1, sha)
	assert.Error(t, err)
}

func TestGitHubClient_GetCommitByWrongSha(t *testing.T) {
	ctx := context.Background()
	sha := "5dcb09b5b57875f334f61aebed695e2e4193db5e"
	response := []byte(`{
		"message": "No commit found for SHA: 5dcb09b5b57875f334f61aebed695e2e4193db5e"
	}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits/%s", owner, repo1, sha),
		http.StatusUnprocessableEntity, createGitHubHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No commit found for SHA: 5dcb09b5b57875f334f61aebed695e2e4193db5e")
	assert.Empty(t, result)
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
			RepositoryVisibility: Public,
			CloneInfo:            CloneInfo{HTTP: "https://github.com/octocat/Hello-World.git", SSH: "git@github.com:octocat/Hello-World.git"},
		},
		info,
	)

	_, err = createBadGitHubClient(t).GetRepositoryInfo(ctx, "octocat", "Hello-World")
	assert.Error(t, err)
}

func TestGitHubClient_CreateLabel(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Label{}, fmt.Sprintf("/repos/jfrog/%s/labels", repo1), createGitHubHandler)
	defer cleanUp()

	err := client.CreateLabel(ctx, owner, repo1, LabelInfo{
		Name:        labelName,
		Description: "label-description",
		Color:       "001122",
	})
	assert.NoError(t, err)

	err = createBadGitHubClient(t).CreateLabel(ctx, owner, repo1, LabelInfo{
		Name:        labelName,
		Description: "label-description",
		Color:       "001122",
	})
	assert.Error(t, err)
}

func TestGitHubClient_GetLabel(t *testing.T) {
	ctx := context.Background()

	expectedLabel := &LabelInfo{Name: labelName, Description: "label-description", Color: "001122"}
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false,
		github.Label{Name: &expectedLabel.Name, Description: &expectedLabel.Description, Color: &expectedLabel.Color},
		fmt.Sprintf("/repos/jfrog/%s/labels/%s", repo1, url.PathEscape(expectedLabel.Name)), createGitHubHandler)
	defer cleanUp()

	actualLabel, err := client.GetLabel(ctx, owner, repo1, expectedLabel.Name)
	assert.NoError(t, err)

	assert.Equal(t, expectedLabel, actualLabel)
	_, err = createBadGitHubClient(t).GetLabel(ctx, owner, repo1, expectedLabel.Name)
	assert.Error(t, err)
}

func TestGitGubClient_GetLabelNotExisted(t *testing.T) {
	ctx := context.Background()

	client, cleanUp := createBodyHandlingServerAndClient(t, vcsutils.GitHub, false, github.Label{},
		fmt.Sprintf("/repos/jfrog/%s/labels/%s", repo1, "not-existed"), http.StatusNotFound, []byte{}, "GET", createGitHubWithBodyHandler)
	defer cleanUp()

	actualLabel, err := client.GetLabel(ctx, owner, repo1, "not-existed")
	assert.NoError(t, err)
	assert.Nil(t, actualLabel)
}

func TestGitHubClient_ListPullRequestLabels(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []*github.Label{{Name: &labelName}}, "/repos/jfrog/repo-1/issues/1/labels", createGitHubHandler)
	defer cleanUp()

	labels, err := client.ListPullRequestLabels(ctx, owner, repo1, 1)
	assert.NoError(t, err)
	assert.Len(t, labels, 1)
	assert.Equal(t, labelName, labels[0])

	_, err = createBadGitHubClient(t).ListPullRequestLabels(ctx, owner, repo1, 1)
	assert.Error(t, err)
}

func TestGitHubClient_ListOpenPullRequests(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "pull_requests_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/pulls?state=open", owner, repo1), createGitHubHandler)
	defer cleanUp()

	result, err := client.ListOpenPullRequests(ctx, owner, repo1)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(PullRequestInfo{
		ID:     1,
		Source: BranchInfo{Name: "new-topic", Repository: "Hello-World"},
		Target: BranchInfo{Name: "master", Repository: "Hello-World"},
	}, result[0]))

	_, err = createBadGitHubClient(t).ListPullRequestComments(ctx, owner, repo1, 1)
	assert.Error(t, err)
}

func TestGitHubClient_GetPullRequestInfoById(t *testing.T) {
	ctx := context.Background()
	pullRequestId := 1
	repoName := "Hello-World"
	response, err := os.ReadFile(filepath.Join("testdata", "github", "pull_request_info_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repoName, pullRequestId), createGitHubHandler)
	defer cleanUp()

	result, err := client.GetPullRequestInfoById(ctx, owner, repoName, pullRequestId)
	require.NoError(t, err)
	assert.True(t, reflect.DeepEqual(PullRequestInfo{
		ID:     1,
		Source: BranchInfo{Name: "new-topic", Repository: "Hello-World"},
		Target: BranchInfo{Name: "master", Repository: "Hello-World"},
	}, result))

	_, err = createBadGitHubClient(t).ListPullRequestComments(ctx, owner, repoName, 1)
	assert.Error(t, err)
}

func TestGitHubClient_ListPullRequestComments(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "pull_request_comments_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/issues/1/comments", owner, repo1), createGitHubHandler)
	defer cleanUp()

	result, err := client.ListPullRequestComments(ctx, owner, repo1, 1)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	expectedCreated, err := time.Parse(time.RFC3339, "2011-04-14T16:00:49Z")
	assert.NoError(t, err)
	assert.Equal(t, CommentInfo{
		ID:      10,
		Content: "Great stuff!",
		Created: expectedCreated,
	}, result[0])

	_, err = createBadGitHubClient(t).ListPullRequestComments(ctx, owner, repo1, 1)
	assert.Error(t, err)
}

func TestGitHubClient_UnlabelPullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, &github.PullRequest{}, fmt.Sprintf("/repos/jfrog/repo-1/issues/1/labels/%s", url.PathEscape(labelName)), createGitHubHandler)
	defer cleanUp()

	err := client.UnlabelPullRequest(ctx, owner, repo1, labelName, 1)
	assert.NoError(t, err)

	err = createBadGitHubClient(t).UnlabelPullRequest(ctx, owner, repo1, labelName, 1)
	assert.Error(t, err)
}

func TestGitHubClient_UploadScanningAnalysis(t *testing.T) {
	ctx := context.Background()
	scan := "{\n    \"version\": \"2.1.0\",\n    \"$schema\": \"https://json.schemastore.org/sarif-2.1.0-rtm.5.json\",\n    \"runs\": [\n      {\n        \"tool\": {\n          \"driver\": {\n            \"informationUri\": \"https://jfrog.com/xray/\",\n            \"name\": \"Xray\",\n            \"rules\": [\n              {\n                \"id\": \"XRAY-174176\",\n                \"shortDescription\": null,\n                \"fullDescription\": {\n                  \"text\": \"json Package for Node.js lib/json.js _parseString() Function -d Argument Handling Local Code Execution Weakness\"\n                },\n                \"properties\": {\n                  \"security-severity\": \"8\"\n                }\n              }\n            ]\n          }\n        },\n        \"results\": [\n          {\n            \"ruleId\": \"XRAY-174176\",\n            \"ruleIndex\": 1,\n            \"message\": {\n              \"text\": \"json 9.0.6. Fixed in Versions: [11.0.0]\"\n            },\n            \"locations\": [\n              {\n                \"physicalLocation\": {\n                  \"artifactLocation\": {\n                    \"uri\": \"package.json\"\n                  }\n                }\n              }\n            ]\n          }\n        ]\n      }\n    ]\n  }"
	response, err := os.ReadFile(filepath.Join("testdata", "github", "commit_list_response.json"))
	assert.NoError(t, err)
	expectedUploadSarifID := "b16b0368-01b9-11ed-90a3-cabff0b8ad31"
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=1&sha=master", owner, repo1), createGitHubSarifUploadHandler)
	defer cleanUp()

	sarifID, err := client.UploadCodeScanning(ctx, owner, repo1, "master", scan)
	assert.NoError(t, err)
	assert.Equal(t, expectedUploadSarifID, sarifID)

	_, err = createBadGitHubClient(t).UploadCodeScanning(ctx, owner, repo1, "master", scan)
	assert.Error(t, err)
}

func TestGitHubClient_GetRepositoryEnvironmentInfo(t *testing.T) {
	ctx := context.Background()

	response, err := os.ReadFile(filepath.Join("testdata", "github", "repository_environment_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response, fmt.Sprintf("/repos/jfrog/repo-1/environments/%s", envName), createGitHubHandler)
	defer cleanUp()

	repositoryEnvironmentInfo, err := client.GetRepositoryEnvironmentInfo(ctx, owner, repo1, envName)
	assert.NoError(t, err)
	assert.Equal(t, envName, repositoryEnvironmentInfo.Name)
	assert.Equal(t, "https://api.github.com/repos/superfrog/test-repo/environments/frogbot", repositoryEnvironmentInfo.Url)
	assert.Equal(t, []string{"superfrog"}, repositoryEnvironmentInfo.Reviewers)

	_, err = createBadGitHubClient(t).GetRepositoryEnvironmentInfo(ctx, owner, repo1, envName)
	assert.Error(t, err)
}

func TestGitHubClient_ExtractGitHubEnvironmentReviewers(t *testing.T) {
	reviewer1, reviewer2 := "reviewer-1", "reviewer-2"
	environment := &github.Environment{
		ProtectionRules: []*github.ProtectionRule{{
			Reviewers: []*github.RequiredReviewer{
				{Reviewer: &repositoryEnvironmentReviewer{Login: reviewer1}},
				{Reviewer: &repositoryEnvironmentReviewer{Login: reviewer2}},
			},
		}},
	}

	actualReviewers, err := extractGitHubEnvironmentReviewers(environment)
	assert.NoError(t, err)
	assert.Equal(t, []string{reviewer1, reviewer2}, actualReviewers)
}

func TestGitHubClient_GetModifiedFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "github", "compare_commits.json"))
		assert.NoError(t, err)

		client, cleanUp := createServerAndClient(
			t,
			vcsutils.GitHub,
			false,
			response,
			"/repos/jfrog/repo-1/compare/sha-1...sha-2?per_page=1",
			createGitHubHandler,
		)
		defer cleanUp()

		fileNames, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		assert.NoError(t, err)
		assert.Equal(t, []string{
			"README.md",
			"vcsclient/azurerepos.go",
			"vcsclient/azurerepos_test.go",
			"vcsclient/bitbucketcloud.go",
			"vcsclient/bitbucketcloud_test.go",
			"vcsclient/bitbucketcommon.go",
			"vcsclient/bitbucketserver.go",
			"vcsclient/bitbucketserver_test.go",
			"vcsclient/common_test.go",
			"vcsclient/github.go",
			"vcsclient/github_test.go",
			"vcsclient/gitlab.go",
			"vcsclient/gitlab_test.go",
			"vcsclient/gitlabcommon.go",
			"vcsclient/testdata/github/repository_environment_response.json",
			"vcsclient/vcsclient.go",
			"vcsclient/vcsclient_old.go",
		},
			fileNames,
		)
	})

	t.Run("validation fails", func(t *testing.T) {
		client := GitHubClient{}
		_, err := client.GetModifiedFiles(ctx, "", repo1, "sha-1", "sha-2")
		require.Equal(t, errors.New("validation failed: required parameter 'owner' is missing"), err)
		_, err = client.GetModifiedFiles(ctx, owner, "", "sha-1", "sha-2")
		require.Equal(t, errors.New("validation failed: required parameter 'repository' is missing"), err)
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "", "sha-2")
		require.Equal(t, errors.New("validation failed: required parameter 'refBefore' is missing"), err)
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "")
		require.Equal(t, errors.New("validation failed: required parameter 'refAfter' is missing"), err)
	})

	t.Run("failed request", func(t *testing.T) {
		client, cleanUp := createServerAndClientReturningStatus(
			t,
			vcsutils.GitHub,
			true,
			nil,
			"/repos/jfrog/repo-1/compare/sha-1...sha-2?per_page=1",
			http.StatusInternalServerError,
			createGitHubHandler,
		)
		defer cleanUp()
		_, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		require.Error(t, err)
		require.Contains(t, err.Error(), "repos/jfrog/repo-1/compare/sha-1...sha-2?per_page=1: 500  []")
	})
}

func TestGitHubClient_TestGetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "5fbf81b31ff7a3b06bd362d1891e2f01bdb2be69"
	t.Run("Empty response", func(t *testing.T) {
		client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, fmt.Sprintf("/repos/jfrog/%s/commits/%s/status", repo1, ref), createGitHubHandler)
		defer cleanUp()
		_, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.NoError(t, err)
	})
	t.Run("Full response", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "github", "commits_statuses.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
			fmt.Sprintf("/repos/jfrog/%s/commits/%s/status", repo1, ref),
			createGitHubHandler)
		defer cleanUp()
		commitStatuses, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.NoError(t, err)
		assert.True(t, len(commitStatuses) == 4)
		assert.True(t, commitStatuses[0].State == Pass)
		assert.True(t, commitStatuses[1].State == InProgress)
		assert.True(t, commitStatuses[2].State == Fail)
		assert.True(t, commitStatuses[3].State == Error)
	})
	t.Run("Bad response format", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "github", "commits_statuses_bad_json.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
			fmt.Sprintf("/repos/jfrog/%s/commits/%s/status", repo1, ref),
			createGitHubHandler)
		defer cleanUp()
		_, err = client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Error(t, err)
		_, err = createBadGitHubClient(t).GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Error(t, err)
	})
	t.Run("Bad client", func(t *testing.T) {
		client := createBadGitHubClient(t)
		_, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Error(t, err)
	})
}

func TestGitHubClient_getGitHubGitRemoteUrl(t *testing.T) {
	testCases := []struct {
		name           string
		apiEndpoint    string
		owner          string
		repo           string
		expectedResult string
	}{
		{
			name:           "GitHub Cloud",
			apiEndpoint:    "https://api.github.com",
			owner:          "my-org",
			repo:           "my-repo",
			expectedResult: "https://github.com/my-org/my-repo.git",
		},
		{
			name:           "GitHub On-Premises",
			apiEndpoint:    "https://github.example.com/api/v3",
			owner:          "my-org",
			repo:           "my-repo",
			expectedResult: "https://github.example.com/api/v3/my-org/my-repo.git",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := VcsInfo{APIEndpoint: tc.apiEndpoint}
			client, err := NewGitHubClient(info, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResult, getGitHubGitRemoteUrl(client, tc.owner, tc.repo))
		})
	}
}

func createBadGitHubClient(t *testing.T) VcsClient {
	client, err := NewClientBuilder(vcsutils.GitHub).ApiEndpoint("https://bad^endpoint").Build()
	require.NoError(t, err)
	return client
}

func createGitHubWithBodyHandler(t *testing.T, expectedURI string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHttpMethod string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, expectedHttpMethod, request.Method)
		assert.Equal(t, expectedURI, request.RequestURI)
		assert.Equal(t, "Bearer "+token, request.Header.Get("Authorization"))

		b, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Equal(t, expectedRequestBody, b)

		writer.WriteHeader(expectedStatusCode)
		_, err = writer.Write(response)
		assert.NoError(t, err)
	}
}

func createGitHubWithPaginationHandler(t *testing.T, _ string, response []byte, _ []byte, expectedStatusCode int, expectedHttpMethod string) http.HandlerFunc {
	var repos []github.Repository
	err := json.Unmarshal(response, &repos)
	assert.NoError(t, err)
	const (
		defaultPerPage = 30
		perPageKey     = "perPage"
		pageKey        = "page"
		link           = "Link"
	)
	count := len(repos)
	return func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, expectedHttpMethod, request.Method)
		pageSize := defaultPerPage
		page := 1
		uri, err := url.Parse(request.RequestURI)
		assert.NoError(t, err)
		if uri.Query().Has(perPageKey) {
			pageSize, err = strconv.Atoi(uri.Query().Get(perPageKey))
			assert.NoError(t, err)
		}
		if uri.Query().Has(pageKey) {
			page, err = strconv.Atoi(uri.Query().Get(pageKey))
			assert.NoError(t, err)
			if page <= 0 {
				page = 1
			}
		}

		lastPage := int(math.Ceil(float64(count) / float64(pageSize)))
		lastLink := fmt.Sprintf("<https://api.github.com/user/repos?page=%v>; rel=\"last\"", lastPage) //https://docs.github.com/en/rest/guides/traversing-with-pagination

		writer.Header().Add(link, lastLink)
		writer.WriteHeader(expectedStatusCode)

		var pageItems []github.Repository
		if page <= lastPage {
			low := (page - 1) * pageSize
			high := page * pageSize
			if (count - page*pageSize) < 0 {
				high = count
			}
			pageItems = repos[low:high]
		}

		pageResponse, err := json.Marshal(pageItems)
		assert.NoError(t, err)

		_, err = writer.Write(pageResponse)
		assert.NoError(t, err)
	}
}

func createGitHubHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedURI, r.RequestURI)
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

func createGitHubSarifUploadHandler(t *testing.T, _ string, _ []byte, _ int) http.HandlerFunc {
	resultSHA := "66d9a06b02a9f3f5fb47bb026a6fa5577647d96e"
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		if r.RequestURI == "/repos/jfrog/repo-1/commits?page=1&per_page=1&sha=master" {
			w.WriteHeader(200)
			repositoryCommits := []*github.RepositoryCommit{
				{
					SHA: &resultSHA,
				},
			}
			jsonRepositoryCommits, err := json.Marshal(repositoryCommits)
			require.NoError(t, err)
			_, err = w.Write(jsonRepositoryCommits)
			require.NoError(t, err)
		} else if r.RequestURI == "/repos/jfrog/repo-1/code-scanning/sarifs" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			bodyAsString := string(body)
			if !strings.Contains(bodyAsString, resultSHA) {
				assert.Fail(t, "Unexpected Commit SHA")
			}
			w.WriteHeader(200)
			_, err = w.Write([]byte(`{"id" : "b16b0368-01b9-11ed-90a3-cabff0b8ad31", "url": ""}`))
			require.NoError(t, err)
		} else {
			assert.Fail(t, "Unexpected Request URI", r.RequestURI)
		}
	}
}
