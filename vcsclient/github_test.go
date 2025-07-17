package vcsclient

import (
	"bytes"
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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v62/github"
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
		http.StatusOK, nil, http.MethodGet, createGitHubWithPaginationHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(repos), len(actualRepositories[username]))
	assert.Equal(t, repoNames, actualRepositories[username])

	// Test Case 2 - No Items to return
	repos = make([]github.Repository, 0)
	client, cleanUp = createBodyHandlingServerAndClient(t, vcsutils.GitHub, false, repos, "/user/repos",
		http.StatusOK, nil, http.MethodGet, createGitHubWithPaginationHandler)
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
	id := rand.Int63() // #nosec G404
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
	id := rand.Int63() // #nosec G404
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
	id := rand.Int63() // #nosec G404
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
	defer func() { assert.NoError(t, vcsutils.RemoveTempDir(dir)) }()

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitHub, false,
		[]byte("https://github.com/octocat/Hello-World/archive/refs/heads/master.tar.gz"),
		"/repos/jfrog/Hello-World/tarball/test", http.StatusFound, createDownloadRepositoryGitHubHandler)
	defer cleanUp()
	assert.NoError(t, err)

	err = client.DownloadRepository(ctx, owner, "Hello-World", "test", dir)
	assert.NoError(t, err)
	fileinfo, err := os.ReadDir(dir)
	assert.NoError(t, err)
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

func TestGitHubClient_UpdatePullRequest(t *testing.T) {
	pullRequestId := 3
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.PullRequest{}, fmt.Sprintf("/repos/jfrog/repo-1/pulls/%v", pullRequestId), createGitHubHandler)
	defer cleanUp()

	err := client.UpdatePullRequest(ctx, owner, repo1, "title", "body", "", pullRequestId, vcsutils.Open)
	assert.NoError(t, err)

	err = client.UpdatePullRequest(ctx, owner, repo1, "title", "body", "master", pullRequestId, vcsutils.Open)
	assert.NoError(t, err)

	err = createBadGitHubClient(t).UpdatePullRequest(ctx, owner, repo1, "title", "body", "master", pullRequestId, vcsutils.Open)
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

func TestGitHubClient_AddPullRequestReviewComments(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.PullRequestReview{}, "/repos/jfrog/repo-1/pulls/1/comments", createAddPullRequestReviewCommentHandler)
	defer cleanUp()

	err := client.AddPullRequestReviewComments(ctx, owner, repo1, 1, []PullRequestComment{
		{
			CommentInfo: CommentInfo{Content: "test1"},
			PullRequestDiff: PullRequestDiff{
				NewFilePath:  "requirements.txt",
				NewStartLine: 3,
			},
		},
		{
			CommentInfo: CommentInfo{Content: "test2"},
			PullRequestDiff: PullRequestDiff{
				NewFilePath:  "requirements.txt",
				NewStartLine: 1,
			},
		},
	}...)
	assert.NoError(t, err)

	err = createBadGitHubClient(t).AddPullRequestReviewComments(ctx, owner, repo1, 1, PullRequestComment{
		CommentInfo: CommentInfo{Content: "test1"},
		PullRequestDiff: PullRequestDiff{
			NewFilePath:  "requirements.txt",
			NewStartLine: 3,
		},
	})
	assert.Error(t, err)
}

func TestGitHubClient_ListPullRequestReviewComments(t *testing.T) {
	ctx := context.Background()
	id := int64(1)
	body := "test"
	created := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []*github.PullRequestComment{{ID: &id, Body: &body, CreatedAt: &github.Timestamp{Time: created}}}, "/repos/jfrog/repo-1/pulls/1/comments", createGitHubHandler)
	defer cleanUp()

	commentInfo, err := client.ListPullRequestReviewComments(ctx, owner, repo1, 1)
	assert.NoError(t, err)
	assert.Len(t, commentInfo, 1)
	assert.Equal(t, id, commentInfo[0].ID)
	assert.Equal(t, body, commentInfo[0].Content)
	assert.Equal(t, created, commentInfo[0].Created)

	commentInfo, err = createBadGitHubClient(t).ListPullRequestReviewComments(ctx, owner, repo1, 1)
	assert.Empty(t, commentInfo)
	assert.Error(t, err)
}

func TestGitHubClient_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=50&sha=master", owner, repo1), createGitHubHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "6dcb09b5b57875f334f61aebed695e2e4193db5e",
		AuthorName:    "Monalisa Octocat",
		CommitterName: "Joconde Octocat",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"6dcb09b5b57875f334f61aebed695e2e4193db5e"},
		AuthorEmail:   "support@github.com",
	}, result)

	_, err = createBadGitHubClient(t).GetLatestCommit(ctx, owner, repo1, "master")
	assert.Error(t, err)
}

func TestGitHubClient_GetCommits(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=50&sha=master", owner, repo1), createGitHubHandler)
	defer cleanUp()

	result, err := client.GetCommits(ctx, owner, repo1, "master")

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "6dcb09b5b57875f334f61aebed695e2e4193db5e",
		AuthorName:    "Monalisa Octocat",
		CommitterName: "Joconde Octocat",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"6dcb09b5b57875f334f61aebed695e2e4193db5e"},
		AuthorEmail:   "support@github.com",
	}, result[0])
	assert.Equal(t, CommitInfo{
		Hash:          "6dcb09b5b57875f334f61aebed695e2e4193db5e",
		AuthorName:    "Leonardo De Vinci",
		CommitterName: "Leonardo De Vinci",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"6dcb09b5b57875f334f61aebed695e2e4193db5e"},
		AuthorEmail:   "vinci@github.com",
	}, result[1])

	_, err = createBadGitHubClient(t).GetCommits(ctx, owner, repo1, "master")
	assert.Error(t, err)
}

func TestGitHubClient_GetCommitsWithQueryOptions(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "commit_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=30&since=2021-01-01T00%%3A00%%3A00Z&until=", owner, repo1), createGitHubHandlerForUnknownUrl)
	defer cleanUp()

	options := GitCommitsQueryOptions{
		Since: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		ListOptions: ListOptions{
			Page:    1,
			PerPage: 30,
		},
	}
	result, err := client.GetCommitsWithQueryOptions(ctx, owner, repo1, options)

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "6dcb09b5b57875f334f61aebed695e2e4193db5e",
		AuthorName:    "Monalisa Octocat",
		CommitterName: "Joconde Octocat",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"6dcb09b5b57875f334f61aebed695e2e4193db5e"},
		AuthorEmail:   "support@github.com",
	}, result[0])
	assert.Equal(t, CommitInfo{
		Hash:          "6dcb09b5b57875f334f61aebed695e2e4193db5e",
		AuthorName:    "Leonardo De Vinci",
		CommitterName: "Leonardo De Vinci",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"6dcb09b5b57875f334f61aebed695e2e4193db5e"},
		AuthorEmail:   "vinci@github.com",
	}, result[1])

	_, err = createBadGitHubClient(t).GetCommitsWithQueryOptions(ctx, owner, repo1, options)
	assert.Error(t, err)
}

func TestGitHubClient_GetLatestCommitNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
    	"documentation_url": "https://docs.github.com/rest/reference/repos#list-commits",
    	"message": "Not Found"
	}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=50&sha=master", owner, "unknown"), http.StatusNotFound,
		createGitHubHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, "unknown", "master")

	assert.Error(t, err)
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
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=50&sha=unknown", owner, repo1), http.StatusNotFound,
		createGitHubHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "unknown")

	assert.Error(t, err)
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
	assert.NoError(t, err)

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

	assert.NoError(t, err)
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

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          sha,
		AuthorName:    "Monalisa Octocat",
		CommitterName: "Joconde Octocat",
		Url:           "https://api.github.com/repos/octocat/Hello-World/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		Timestamp:     1302796850,
		Message:       "Fix all the bugs",
		ParentHashes:  []string{"5dcb09b5b57875f334f61aebed695e2e4193db5e"},
		AuthorEmail:   "support@github.com",
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
	assert.Error(t, err)
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
	assert.NoError(t, err)
	assert.Equal(t,
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
		fmt.Sprintf("/repos/jfrog/%s/labels/%s", repo1, "not-existed"), http.StatusNotFound, []byte{}, http.MethodGet, createGitHubWithBodyHandler)
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
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, PullRequestInfo{
		ID:     1347,
		Title:  "Amazing new feature",
		Author: "octocat",
		Source: BranchInfo{Name: "new-topic", Repository: "Hello-World", Owner: owner},
		Target: BranchInfo{Name: "master", Repository: "Hello-World", Owner: owner},
		URL:    "https://github.com/octocat/Hello-World/pull/1347",
		Status: "open",
	}, result[0])

	_, err = createBadGitHubClient(t).ListPullRequestComments(ctx, owner, repo1, 1)
	assert.Error(t, err)

	// With body:
	result, err = client.ListOpenPullRequestsWithBody(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, PullRequestInfo{
		ID:     1347,
		Title:  "Amazing new feature",
		Body:   "hello world",
		Author: "octocat",
		Source: BranchInfo{Name: "new-topic", Repository: "Hello-World", Owner: owner},
		Target: BranchInfo{Name: "master", Repository: "Hello-World", Owner: owner},
		URL:    "https://github.com/octocat/Hello-World/pull/1347",
		Status: "open",
	}, result[0])

	_, err = createBadGitHubClient(t).ListPullRequestComments(ctx, owner, repo1, 1)
	assert.Error(t, err)
}

func TestGitHubClient_GetPullRequestByID(t *testing.T) {
	ctx := context.Background()
	pullRequestId := 1347
	repoName := "Hello-World"
	forkedOwner := owner + "Forked"

	// Successful response
	// This response mimics a pull request from a forked source
	response, err := os.ReadFile(filepath.Join("testdata", "github", "pull_request_info_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repoName, pullRequestId), createGitHubHandler)
	defer cleanUp()
	result, err := client.GetPullRequestByID(ctx, owner, repoName, pullRequestId)
	assert.NoError(t, err)
	assert.EqualValues(t, PullRequestInfo{
		ID:     int64(pullRequestId),
		Title:  "Amazing new feature",
		Source: BranchInfo{Name: "new-topic", Repository: "Hello-World", Owner: owner},
		Target: BranchInfo{Name: "master", Repository: "Hello-World", Owner: forkedOwner},
		URL:    "https://github.com/octocat/Hello-World/pull/1347",
		Author: "octocat",
		Status: "open",
	}, result)

	// Bad Labels
	badLabels, err := os.ReadFile(filepath.Join("testdata", "github", "pull_request_info_response_bad_labels.json"))
	assert.NoError(t, err)
	badLabelsClient, badLabelClientCleanUp := createServerAndClient(t, vcsutils.GitHub, false, badLabels,
		fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repoName, pullRequestId), createGitHubHandler)
	defer badLabelClientCleanUp()
	_, err = badLabelsClient.GetPullRequestByID(ctx, owner, repoName, pullRequestId)
	assert.Error(t, err)

	// Bad client
	_, err = createBadGitHubClient(t).GetPullRequestByID(ctx, owner, repoName, pullRequestId)
	assert.Error(t, err)

	// Bad Response
	badResponseClient, badResponseCleanUp := createServerAndClient(t, vcsutils.GitHub, false, "{",
		fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repoName, pullRequestId), createGitHubHandler)
	defer badResponseCleanUp()
	_, err = badResponseClient.GetPullRequestByID(ctx, owner, repoName, pullRequestId)
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
	assert.NoError(t, err)
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

func TestGitHubClient_ListPullRequestReviews(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "pull_request_reviews_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo1, 1), createGitHubHandler)
	defer cleanUp()

	result, err := client.ListPullRequestReviews(ctx, owner, repo1, 1)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, PullRequestReviewDetails{
		ID:          80,
		Reviewer:    "octocat",
		Body:        "This is close to perfect! Please address the suggested inline change.",
		SubmittedAt: "2019-11-17 17:43:43 +0000 UTC",
		CommitID:    "ecdd80bb57125d7ba9641ffaa4d7d2c19d3f3091",
		State:       "CHANGES_REQUESTED",
	}, result[0])

	_, err = createBadGitHubClient(t).ListPullRequestReviews(ctx, owner, repo1, 1)
	assert.Error(t, err)
}

func TestGitHubClient_ListPullRequestsAssociatedWithCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "github", "pull_requests_associated_with_commit_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits/%s/pulls", owner, repo1, "commitSHA"), createGitHubHandler)
	defer cleanUp()

	result, err := client.ListPullRequestsAssociatedWithCommit(ctx, owner, repo1, "commitSHA")
	assert.NoError(t, err)
	assert.Len(t, result, 1)

	expected := PullRequestInfo{
		ID:   1347,
		Body: "",
		URL:  "https://github.com/octocat/Hello-World/pull/1347",
		Source: BranchInfo{
			Name:       "new-topic",
			Repository: "Hello-World",
			Owner:      "octocat",
		},
		Target: BranchInfo{
			Name:       "master",
			Repository: "Hello-World",
			Owner:      "octocat",
		},
	}

	assert.Equal(t, expected.ID, result[0].ID)
	assert.Equal(t, expected.Body, result[0].Body)
	assert.Equal(t, expected.URL, result[0].URL)
	assert.Equal(t, expected.Source, result[0].Source)
	assert.Equal(t, expected.Target, result[0].Target)

	_, err = createBadGitHubClient(t).ListPullRequestsAssociatedWithCommit(ctx, owner, repo1, "commitSHA")
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
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response,
		fmt.Sprintf("/repos/%s/%s/commits?page=1&per_page=50&sha=master", owner, repo1), createGitHubSarifUploadHandler)
	defer cleanUp()

	sarifID, err := client.UploadCodeScanning(ctx, owner, repo1, "master", scan)
	assert.NoError(t, err)
	assert.Equal(t, "", sarifID)

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
		assert.EqualError(t, err, "validation failed: required parameter 'owner' is missing")
		_, err = client.GetModifiedFiles(ctx, owner, "", "sha-1", "sha-2")
		assert.EqualError(t, err, "validation failed: required parameter 'repository' is missing")
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "", "sha-2")
		assert.EqualError(t, err, "validation failed: required parameter 'refBefore' is missing")
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "")
		assert.EqualError(t, err, "validation failed: required parameter 'refAfter' is missing")
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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repos/jfrog/repo-1/compare/sha-1...sha-2?per_page=1: 500  []")
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
		assert.Len(t, commitStatuses, 4)
		assert.Equal(t, Pass, commitStatuses[0].State)
		assert.Equal(t, InProgress, commitStatuses[1].State)
		assert.Equal(t, Fail, commitStatuses[2].State)
		assert.Equal(t, Error, commitStatuses[3].State)
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

func TestGitHubClient_DeletePullRequestReviewComments(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, "", createGitHubHandlerWithoutExpectedURI)
	defer cleanUp()
	err := client.DeletePullRequestReviewComments(ctx, owner, repo1, 1, []CommentInfo{{ID: 1}, {ID: 2}}...)
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.DeletePullRequestReviewComments(ctx, owner, repo1, 1, CommentInfo{ID: 1})
	assert.Error(t, err)
}

func TestGitHubClient_DeletePullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, fmt.Sprintf("/repos/%v/%v/issues/comments/1", owner, repo1), createGitHubHandler)
	defer cleanUp()
	err := client.DeletePullRequestComment(ctx, owner, repo1, 1, 1)
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.DeletePullRequestComment(ctx, owner, repo1, 1, 1)
	assert.Error(t, err)
}

func TestGitHubClient_CreateBranch(t *testing.T) {
	ctx := context.Background()
	branchResponse := github.Branch{
		Name:      github.String("master"),
		Commit:    &github.RepositoryCommit{},
		Protected: nil,
	}
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, branchResponse, "", createGitHubHandlerWithoutExpectedURI)
	defer cleanUp()
	err := client.CreateBranch(ctx, owner, repo1, "master", "BranchForTest")
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.CreateBranch(ctx, owner, repo1, "master", "BranchForTest")
	assert.Error(t, err)
}

func TestGitHubClient_AllowWorkflows(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, fmt.Sprintf("/orgs/%v/actions/permissions", owner), createGitHubHandler)
	defer cleanUp()
	err := client.AllowWorkflows(ctx, owner)
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.AllowWorkflows(ctx, owner)
	assert.Error(t, err)
}

func TestGitHubClient_AddOrganizationSecret(t *testing.T) {
	ctx := context.Background()
	publicKeyResponse := github.PublicKey{
		KeyID: github.String("key-id"),
		Key:   github.String("mfB0IZfFzP0YoJ4GzRbGVFfuR6MGlwGTi5jJ6EEXa5g="),
	}
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, publicKeyResponse, "", createGitHubHandlerWithoutExpectedURI)
	defer cleanUp()
	err := client.AddOrganizationSecret(ctx, owner, "super-duper-secret", "super-duper-secret-value")
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.AddOrganizationSecret(ctx, owner, "super-duper-secret", "super-duper-secret-value")
	assert.Error(t, err)
}

func TestGitHubClient_CreateOrgVariable(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.ActionsVariable{},
		"/orgs/jfrog/actions/variables", createGitHubHandler)
	defer cleanUp()

	// Test with default visibility "all"
	err := client.CreateOrgVariable(ctx, owner, "JF_URL", "test.jfrogdev.org")
	assert.NoError(t, err)

	// Test with bad client
	err = createBadGitHubClient(t).CreateOrgVariable(ctx, owner, "JF_URL", "test.jfrogdev.org")
	assert.Error(t, err)
}

func TestGitHubClient_CommitAndPushFiles(t *testing.T) {
	ctx := context.Background()
	sourceBranch := "feature-branch"
	filesToCommit := []FileToCommit{
		{Path: "example.txt", Content: "example content"},
	}
	expectedResponses := map[string]mockGitHubResponse{
		"/repos/jfrog/repo-1/git/ref/heads/feature-branch": {
			StatusCode: 200,
			Response: mustMarshal(&github.Reference{
				Ref: github.String("refs/heads/feature-branch"),
				Object: &github.GitObject{
					SHA: github.String("abc123abc123abc123abc123abc123abc123abcd"),
				},
			}),
		},
		"/repos/jfrog/repo-1/git/commits/abc123abc123abc123abc123abc123abc123abcd": {
			StatusCode: 200,
			Response: mustMarshal(&github.Commit{
				SHA: github.String("abc123abc123abc123abc123abc123abc123abcd"),
				Tree: &github.Tree{
					SHA: github.String("def456def456def456def456def456def456defa"),
				},
			}),
		},
		"/repos/jfrog/repo-1/git/blobs": {
			StatusCode: 201,
			Response: mustMarshal(&github.Blob{
				SHA: github.String("blobsha1234567890abcdef1234567890abcdef1234"),
			}),
		},
		"/repos/jfrog/repo-1/git/trees": {
			StatusCode: 201,
			Response: mustMarshal(&github.Tree{
				SHA: github.String("tree789tree789tree789tree789tree789tree789"),
			}),
		},
		"/repos/jfrog/repo-1/git/commits": {
			StatusCode: 201,
			Response: mustMarshal(&github.Commit{
				SHA: github.String("commit123commit123commit123commit123commit123"),
			}),
		},
		"/repos/jfrog/repo-1/git/refs/heads/feature-branch": {
			StatusCode: 200,
			Response: mustMarshal(&github.Reference{
				Ref: github.String("refs/heads/feature-branch"),
				Object: &github.GitObject{
					SHA: github.String("commit123commit123commit123commit123commit123"),
				},
			}),
		},
	}

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, "", createGitHubHandlerWithMultiResponse(t, expectedResponses))
	defer cleanUp()
	err := client.CommitAndPushFiles(ctx, owner, repo1, sourceBranch, "generic commit message", "example", "example@jfrog.com", filesToCommit)
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.CommitAndPushFiles(ctx, owner, repo1, sourceBranch, "generic commit message", "example", "example@jfrog.com", filesToCommit)
	assert.Error(t, err)
}

func TestGitHubClient_GetRepoCollaborators(t *testing.T) {
	ctx := context.Background()
	response := []*github.User{
		{Login: github.String("example")},
	}
	affiliation := "direct"
	permission := "maintain"
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response, fmt.Sprintf("/repos/%v/%v/collaborators?affiliation=%v&permission=%v", owner, repo1, affiliation, permission), createGitHubHandler)
	defer cleanUp()
	collaborators, err := client.GetRepoCollaborators(ctx, owner, repo1, "direct", "maintain")
	assert.NoError(t, err)
	assert.Len(t, collaborators, 1)
	assert.Equal(t, collaborators[0], *response[0].Login)
	client = createBadGitHubClient(t)
	_, err = client.GetRepoCollaborators(ctx, owner, repo1, "direct", "maintain")
	assert.Error(t, err)
}

func TestGitHubClient_GetRepoTeamsByPermissions(t *testing.T) {
	ctx := context.Background()
	response := []*github.Team{
		{
			Name:       github.String("dev-team"),
			Slug:       github.String("dev-team"),
			ID:         github.Int64(1234567),
			Permission: github.String("maintain"),
		},
	}
	permissions := []string{"maintain"}

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, response, fmt.Sprintf("/repos/%v/%v/teams", owner, repo1), createGitHubHandler)
	defer cleanUp()
	teams, err := client.GetRepoTeamsByPermissions(ctx, owner, repo1, permissions)
	assert.NoError(t, err)
	assert.Len(t, teams, 1)
	assert.Equal(t, teams[0], response[0].GetID())
	client = createBadGitHubClient(t)
	_, err = client.GetRepoTeamsByPermissions(ctx, owner, repo1, permissions)
	assert.Error(t, err)
}

func TestGitHubClient_CreateOrUpdateEnvironment(t *testing.T) {
	ctx := context.Background()
	teams := []int64{123467}
	environment := "frogbot"

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, fmt.Sprintf("/repos/%v/%v/environments/%v", owner, repo1, environment), createGitHubHandler)
	defer cleanUp()
	err := client.CreateOrUpdateEnvironment(ctx, owner, repo1, environment, teams, nil)
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.CreateOrUpdateEnvironment(ctx, owner, repo1, environment, teams, nil)
	assert.Error(t, err)
}

func TestGitHubClient_MergePullRequest(t *testing.T) {
	ctx := context.Background()
	prNumber := 1
	commitMessage := "merge"

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, fmt.Sprintf("/repos/%v/%v/pulls/%v/%v", owner, repo1, prNumber, commitMessage), createGitHubHandler)
	defer cleanUp()
	err := client.MergePullRequest(ctx, owner, repo1, prNumber, commitMessage)
	assert.NoError(t, err)
	client = createBadGitHubClient(t)
	err = client.MergePullRequest(ctx, owner, repo1, prNumber, commitMessage)
	assert.Error(t, err)
}

func TestGitHubClient_CreatePullRequestDetailed(t *testing.T) {
	ctx := context.Background()
	expectedURL := "https://github.com/jfrog/repo1/pull/875"
	expectedPrNumber := 1234
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.PullRequest{Number: github.Int(expectedPrNumber), HTMLURL: github.String(expectedURL)}, "/repos/jfrog/repo-1/pulls", createGitHubHandler)
	defer cleanUp()

	prInfo, err := client.CreatePullRequestDetailed(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
	assert.Equal(t, expectedPrNumber, prInfo.Number)
	assert.Equal(t, expectedURL, prInfo.URL)

	_, err = createBadGitHubClient(t).CreatePullRequestDetailed(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.Error(t, err)
}

func createBadGitHubClient(t *testing.T) VcsClient {
	client, err := NewClientBuilder(vcsutils.GitHub).ApiEndpoint("https://badendpoint").Build()
	assert.NoError(t, err)
	return client
}

func createGitHubWithBodyHandler(t *testing.T, expectedURI string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHttpMethod string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, expectedHttpMethod, request.Method)
		assert.Equal(t, expectedURI, request.RequestURI)
		assert.Equal(t, "Bearer "+token, request.Header.Get("Authorization"))

		b, err := io.ReadAll(request.Body)
		assert.NoError(t, err)
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
		assert.NoError(t, err)
	}
}

// Similar to createGitHubHandler but without checking if the expectedURI is equal to the request URI, only if it contained in the request URI.
func createGitHubHandlerForUnknownUrl(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.RequestURI, expectedURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		if strings.Contains(r.RequestURI, "tarball") {
			w.Header().Add("Location", string(response))
			w.WriteHeader(expectedStatusCode)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
	}
}

func createGitHubHandlerWithoutExpectedURI(t *testing.T, _ string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		if strings.Contains(r.RequestURI, "tarball") {
			w.Header().Add("Location", string(response))
			w.WriteHeader(expectedStatusCode)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
	}
}

func createAddPullRequestReviewCommentHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/repos/jfrog/repo-1/pulls/1/commits" {
			commits, err := os.ReadFile(filepath.Join("testdata", "github", "commit_list_response.json"))
			assert.NoError(t, err)
			_, err = w.Write(commits)
			assert.NoError(t, err)
			return
		}
		assert.Equal(t, expectedURI, r.RequestURI)
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
	}
}

func createDownloadRepositoryGitHubHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/repos/jfrog/Hello-World" {
			repositoryResponse, err := os.ReadFile(filepath.Join("testdata", "github", "repository_response.json"))
			assert.NoError(t, err)
			_, err = w.Write(repositoryResponse)
			assert.NoError(t, err)
			return
		}
		assert.Equal(t, expectedURI, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		if strings.Contains(r.RequestURI, "tarball") {
			w.Header().Add("Location", string(response))
			w.WriteHeader(expectedStatusCode)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
	}
}

func createGitHubSarifUploadHandler(t *testing.T, _ string, _ []byte, _ int) http.HandlerFunc {
	resultSHA := "66d9a06b02a9f3f5fb47bb026a6fa5577647d96e"
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/repos/jfrog/repo-1/commits?page=1&per_page=50&sha=master":
			w.WriteHeader(http.StatusOK)
			repositoryCommits := []*github.RepositoryCommit{
				{
					SHA: &resultSHA,
				},
			}
			jsonRepositoryCommits, err := json.Marshal(repositoryCommits)
			assert.NoError(t, err)
			_, err = w.Write(jsonRepositoryCommits)
			assert.NoError(t, err)
		case "/repos/jfrog/repo-1/code-scanning/sarifs":
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			bodyAsString := string(body)
			if !strings.Contains(bodyAsString, resultSHA) {
				assert.Fail(t, "Unexpected Commit SHA")
			}
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{"id" : "b16b0368-01b9-11ed-90a3-cabff0b8ad31", "url": ""}`))
			assert.NoError(t, err)
		default:
			assert.Fail(t, "Unexpected Request URI", r.RequestURI)
		}
	}
}

func TestShouldRetryIfRateLimitExceeded(t *testing.T) {
	// Test case 1: ghResponse is nil
	toRetry := shouldRetryIfRateLimitExceeded(nil, nil)
	assert.False(t, toRetry)

	// Test case 2: ghResponse StatusCode is not rate limit related
	mockResponse := &github.Response{
		Response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("")))},
	}
	toRetry = shouldRetryIfRateLimitExceeded(mockResponse, nil)
	assert.False(t, toRetry)

	// Test case 3: Request error indicates rate limit abuse
	mockResponse.StatusCode = http.StatusTooManyRequests
	var abuseRateLimitErr *github.AbuseRateLimitError
	toRetry = shouldRetryIfRateLimitExceeded(mockResponse, abuseRateLimitErr)
	assert.False(t, toRetry)

	// Test case 4: Response body contains 'rate limit'
	mockResponse.StatusCode = http.StatusForbidden
	mockResponse.Body = io.NopCloser(bytes.NewReader([]byte("This response contains rate limit")))
	toRetry = shouldRetryIfRateLimitExceeded(mockResponse, nil)
	assert.True(t, toRetry)
}

func TestIsRateLimitAbuseError(t *testing.T) {
	// type `Error`, should return false
	isRateLimitAbuseErr := isRateLimitAbuseError(errors.New("hello"))
	assert.False(t, isRateLimitAbuseErr)

	// type `RateLimitError`, should return true
	isRateLimitAbuseErr = isRateLimitAbuseError(&github.RateLimitError{})
	assert.True(t, isRateLimitAbuseErr)

	// type `AbuseRateLimitError`, should return true
	isRateLimitAbuseErr = isRateLimitAbuseError(&github.AbuseRateLimitError{})
	assert.True(t, isRateLimitAbuseErr)
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}

type mockGitHubResponse struct {
	StatusCode int
	Response   []byte
}

func createGitHubHandlerWithMultiResponse(t *testing.T, expectedResponses map[string]mockGitHubResponse) func(*testing.T, string, []byte, int) http.HandlerFunc {
	return func(_ *testing.T, _ string, _ []byte, _ int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Look up the expected response for the URI
			resp, ok := expectedResponses[r.RequestURI]
			assert.True(t, ok, "unexpected URI: %s", r.RequestURI)

			// Check the Authorization header
			assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))

			// Handle special case for "tarball"
			if strings.Contains(r.RequestURI, "tarball") {
				w.Header().Add("Location", string(resp.Response))
				w.WriteHeader(resp.StatusCode)
				return
			}

			// Write the response for the general case
			w.WriteHeader(resp.StatusCode)
			_, err := w.Write(resp.Response)
			assert.NoError(t, err)
		}
	}
}

func TestGitHubClient_ListAppRepositories(t *testing.T) {
	ctx := context.Background()
	response := `{
	  "total_count": 1,
	  "repositories": [
	    {
	      "id": 1296269,
	      "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
	      "name": "Hello-World",
	      "full_name": "octocat/Hello-World",
	      "owner": {
	        "login": "octocat",
	        "id": 1
	      },
	      "private": false,
	      "description": "This your first repo!",
	      "html_url": "https://github.com/octocat/Hello-World",
	      "clone_url": "https://github.com/octocat/Hello-World.git",
	      "ssh_url": "git@github.com:octocat/Hello-World.git",
	      "default_branch": "main"
	    }
	  ]
	}`

	client, cleanUp := createServerAndClient(
		t,
		vcsutils.GitHub,
		false,
		[]byte(response),
		"/installation/repositories?page=1",
		createGitHubHandler,
	)
	defer cleanUp()

	repos, err := client.ListAppRepositories(ctx)
	assert.NoError(t, err)
	assert.Len(t, repos, 1)
	repoInfo := repos[0]
	assert.Equal(t, int64(1296269), repoInfo.ID)
	assert.Equal(t, "Hello-World", repoInfo.Name)
	assert.Equal(t, "octocat/Hello-World", repoInfo.FullName)
	assert.Equal(t, "octocat", repoInfo.Owner)
	assert.Equal(t, false, repoInfo.Private)
	assert.Equal(t, "This your first repo!", repoInfo.Description)
	assert.Equal(t, "https://github.com/octocat/Hello-World", repoInfo.URL)
	assert.Equal(t, "https://github.com/octocat/Hello-World.git", repoInfo.CloneURL)
	assert.Equal(t, "git@github.com:octocat/Hello-World.git", repoInfo.SSHURL)
	assert.Equal(t, "main", repoInfo.DefaultBranch)

	// Negative test: bad client
	_, err = createBadGitHubClient(t).ListAppRepositories(ctx)
	assert.Error(t, err)
}

func TestGithubClient_UploadSnapshotToDependencyGraph(t *testing.T) {
	ctx := context.Background()
	expectedURI := fmt.Sprintf("/repos/%s/%s/dependency-graph/snapshots", owner, repo1)

	resolvedPackages := make(map[string]*ResolvedDependency)
	resolvedPackages["@actions/core"] = &ResolvedDependency{
		PackageURL:   "pkg:/npm/%40actions/core@1.1.9",
		Relationship: "direct",
		Dependencies: []string{"@actions/http-client"},
	}
	resolvedPackages["@actions/http-client"] = &ResolvedDependency{
		PackageURL:   "pkg:/npm/%40actions/http-client@1.0.1",
		Relationship: "direct",
		Dependencies: []string{"tunnel"},
	}
	resolvedPackages["tunnel"] = &ResolvedDependency{
		PackageURL:   "pkg:/npm/tunnel@0.0.6",
		Relationship: "indirect",
	}

	manifests := make(map[string]*Manifest)
	manifests["package-lock.json"] = &Manifest{
		Name:     "package-lock.json",
		File:     &FileInfo{SourceLocation: "src/package-lock.json"},
		Resolved: resolvedPackages,
	}

	snapshot := SbomSnapshot{
		Version: 0,
		Sha:     "ce587453ced02b1526dfb4cb910479d431683101",
		Ref:     "refs/heads/master",
		Job: &JobInfo{
			Correlator: "my-workflow_my-action-name",
			ID:         "my-run-id",
		},
		Detector:  &DetectorInfo{Name: "frogbot", Version: "1.0.0", Url: "https://github.com/jfrog/frogbot"},
		Scanned:   time.Now(),
		Manifests: manifests,
	}

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, expectedURI, createGitHubHandler)
	defer cleanUp()

	err := client.UploadSnapshotToDependencyGraph(ctx, owner, repo1, snapshot)
	assert.NoError(t, err)

	// Negative test: bad client
	err = createBadGitHubClient(t).UploadSnapshotToDependencyGraph(ctx, owner, repo1, snapshot)
	assert.Error(t, err)
}
