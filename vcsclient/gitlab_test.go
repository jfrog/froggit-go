package vcsclient

import (
	"context"
	"encoding/json"
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
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "projects_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createBodyHandlingServerAndClient(t, vcsutils.GitLab, false, response, "", http.StatusOK, nil, http.MethodGet, createGitLabWithPaginationHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"example-user":             {"example-project"},
		"root":                     {"my-project", "go-micro"},
		"gitlab-instance-ba535d0c": {"Monitoring"},
		"froggit-go":               {"repo21", "repo20", "repo19", "repo18", "repo17", "repo16", "repo15", "repo14", "repo13", "repo12", "repo11", "repo10", "repo9", "repo8", "repo7", "repo6", "repo5", "repo4", "repo3", "repo2", "repo1"},
	}, actualRepositories)
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
	id := rand.Int() // #nosec G404
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	actualID, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", vcsutils.Push,
		vcsutils.PrOpened, vcsutils.PrEdited)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, actualID, strconv.Itoa(id))
}

func TestGitLabClient_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int() // #nosec G404
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks/%d", url.PathEscape(owner+"/"+repo1), id), createGitLabHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", token, strconv.Itoa(id),
		vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected, vcsutils.TagPushed, vcsutils.TagRemoved)
	assert.NoError(t, err)
}

func TestGitLabClient_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int() // #nosec G404
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
	dir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer func() { assert.NoError(t, vcsutils.RemoveTempDir(dir)) }()

	repoFile, err := os.ReadFile(filepath.Join("testdata", "gitlab", "hello-world-main.tar.gz"))
	assert.NoError(t, err)

	ref := "5fbf81b31ff7a3b06bd362d1891e2f01bdb2be69"
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, repoFile, fmt.Sprintf("/api/v4/projects/%s/repository/archive.tar.gz?sha=%s", url.PathEscape(owner+"/"+repo1), ref), createDownloadRepositoryGitLabHandler)
	defer cleanUp()

	err = client.DownloadRepository(ctx, owner, repo1, ref, dir)
	assert.NoError(t, err)
	fileinfo, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, fileinfo, 2)
	assert.Equal(t, ".git", fileinfo[0].Name())
	assert.Equal(t, "README.md", fileinfo[1].Name())
}

func TestGitLabClient_DownloadFileFromRepo(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.File{Content: "SGVsbG8gV29ybGQh"}, fmt.Sprintf("/api/v4/projects/%s/repository/files/hello-world?ref=branch-1", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	content, statusCode, err := client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "hello-world")
	assert.NoError(t, err)
	assert.Equal(t, statusCode, http.StatusOK)
	expected := "Hello World!"
	assert.Equal(t, expected, string(content))
}

func TestGitLabClient_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, &gitlab.MergeRequest{}, fmt.Sprintf("/api/v4/projects/%s/merge_requests", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func TestGitLabClient_UpdatePullRequest(t *testing.T) {
	ctx := context.Background()
	prId := 5
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, &gitlab.MergeRequest{}, fmt.Sprintf("/api/v4/projects/%s/merge_requests/%v", url.PathEscape(owner+"/"+repo1), prId), createGitLabHandler)
	defer cleanUp()

	err := client.UpdatePullRequest(ctx, owner, repo1, "PR title", "PR body", "master", prId, vcsutils.Open)
	assert.NoError(t, err)
	err = client.UpdatePullRequest(ctx, owner, repo1, "PR title", "PR body", "", prId, vcsutils.Open)
	assert.NoError(t, err)
	err = client.UpdatePullRequest(ctx, owner, repo1, "PR title", "PR body", "", prId, vcsutils.Closed)
	assert.NoError(t, err)
	err = client.UpdatePullRequest(ctx, owner, repo1, "PR title", "PR body", "", prId, "default")
	assert.NoError(t, err)
}

func TestGitLabClient_AddPullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, &gitlab.MergeRequest{}, fmt.Sprintf("/api/v4/projects/%s/merge_requests/1/notes", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	err := client.AddPullRequestComment(ctx, owner, repo1, "Comment content", 1)
	assert.NoError(t, err)
}

func TestGitLabClient_AddPullRequestReviewComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, "",
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/1/notes", url.PathEscape(owner+"/"+repo1)), createAddPullRequestReviewCommentGitLabHandler)
	defer cleanUp()
	comments := []PullRequestComment{
		{
			CommentInfo: CommentInfo{Content: "test1"},
			PullRequestDiff: PullRequestDiff{
				OriginalFilePath:  "oldPath",
				OriginalStartLine: 1,
				NewFilePath:       "newPath",
				NewStartLine:      2,
			},
		},
	}
	err := client.AddPullRequestReviewComments(ctx, owner, repo1, 7, comments...)
	// No diff found
	assert.Error(t, err)
	comments = []PullRequestComment{
		{
			CommentInfo: CommentInfo{Content: "test1"},
			PullRequestDiff: PullRequestDiff{
				OriginalFilePath:  "VERSION",
				OriginalStartLine: 1,
				NewFilePath:       "VERSION",
				NewStartLine:      2,
			},
		},
	}
	err = client.AddPullRequestReviewComments(ctx, owner, repo1, 7, comments...)
	assert.NoError(t, err)
}

func TestGitLabClient_ListPullRequestReviewComments(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "merge_request_discussion_items.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/1/discussions", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	result, err := client.ListPullRequestReviewComments(ctx, owner, repo1, 1)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, int64(1126), result[0].ID)
	assert.Equal(t, "discussion text", result[0].Content)
	assert.Equal(t, "2018-03-03 21:54:39.668 +0000 UTC", result[0].Created.String())
	assert.Equal(t, int64(1129), result[1].ID)
	assert.Equal(t, "reply to the discussion", result[1].Content)
	assert.Equal(t, "2018-03-04 13:38:02.127 +0000 UTC", result[1].Created.String())
	assert.Equal(t, int64(1128), result[2].ID)
	assert.Equal(t, "a single comment", result[2].Content)
	assert.Equal(t, "2018-03-04 09:17:22.52 +0000 UTC", result[2].Created.String())
}

func TestGitLabClient_ListPullRequestComments(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "pull_request_comments_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/1/notes", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	result, err := client.ListPullRequestComments(ctx, owner, repo1, 1)
	assert.NoError(t, err)
	expectedCreated, err := time.Parse(time.RFC3339, "2013-10-02T09:56:03Z")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, CommentInfo{
		ID:      305,
		Content: "Text of the comment\r\n",
		Created: expectedCreated,
	}, result[1])
}

func TestGitLabClient_ListOpenPullRequests(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "pull_requests_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		"/api/v4/projects/jfrog%2Frepo-1/merge_requests?scope=all&state=opened", createGitLabHandler)
	defer cleanUp()

	result, err := client.ListOpenPullRequests(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.EqualValues(t, PullRequestInfo{
		ID:     302,
		Source: BranchInfo{Name: "test1", Repository: repo1, Owner: owner},
		Target: BranchInfo{Name: "master", Repository: repo1, Owner: owner},
		URL:    "https://gitlab.example.com/my-group/my-project/merge_requests/1",
	}, result[0])

	// With body
	result, err = client.ListOpenPullRequestsWithBody(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.EqualValues(t, PullRequestInfo{
		ID:     302,
		Body:   "hello world",
		Source: BranchInfo{Name: "test1", Repository: repo1, Owner: owner},
		Target: BranchInfo{Name: "master", Repository: repo1, Owner: owner},
		URL:    "https://gitlab.example.com/my-group/my-project/merge_requests/1",
	}, result[0])
}

func TestGitLabClient_GetPullRequestByID(t *testing.T) {
	ctx := context.Background()
	repoName := "repo"
	pullRequestId := 1

	// Successful response
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "get_merge_request_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d", url.PathEscape(owner+"/"+repoName), pullRequestId), createGitLabHandler)
	defer cleanUp()
	result, err := client.GetPullRequestByID(ctx, owner, repoName, pullRequestId)
	assert.NoError(t, err)
	assert.EqualValues(t, PullRequestInfo{
		ID:     133,
		Source: BranchInfo{Name: "manual-job-rules", Repository: repoName, Owner: owner},
		Target: BranchInfo{Name: "master", Repository: repoName, Owner: owner},
		URL:    "https://gitlab.com/marcel.amirault/test-project/-/merge_requests/133",
	}, result)

	// Bad client
	badClient, badClientCleanUp := createServerAndClient(t, vcsutils.GitLab, false, "",
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d", url.PathEscape(owner+"/"+repoName), pullRequestId), createGitLabHandler)
	defer badClientCleanUp()
	_, err = badClient.GetPullRequestByID(ctx, owner, repoName, pullRequestId)
	assert.Error(t, err)

}

func TestGitLabClient_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=50&ref_name=master",
			url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "ed899a2f4b50b4370feeea94676502b42383c746",
		AuthorName:    "Example User",
		CommitterName: "Administrator",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
		Timestamp:     1348131022,
		Message:       "Replace sanitize with escape once",
		ParentHashes:  []string{"6104942438c14ec7bd21c6cd5bd995272b3faff6"},
		AuthorEmail:   "user@example.com",
	}, result)
}

func TestGitLabClient_GetCommits(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=50&ref_name=master",
			url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	result, err := client.GetCommits(ctx, owner, repo1, "master")

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "ed899a2f4b50b4370feeea94676502b42383c746",
		AuthorName:    "Example User",
		CommitterName: "Administrator",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
		Timestamp:     1348131022,
		Message:       "Replace sanitize with escape once",
		ParentHashes:  []string{"6104942438c14ec7bd21c6cd5bd995272b3faff6"},
		AuthorEmail:   "user@example.com",
	}, result[0])
	assert.Equal(t, CommitInfo{
		Hash:          "6104942438c14ec7bd21c6cd5bd995272b3faff6",
		AuthorName:    "randx",
		CommitterName: "ExampleName",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
		Timestamp:     1348131022,
		Message:       "Sanitize for network graph",
		ParentHashes:  []string{"ae1d9fb46aa2b07ee9836d49862ec4e2c46fbbba"},
		AuthorEmail:   "user@example.com",
	}, result[1])
}

func TestGitLabClient_GetCommitsWithQueryOptions(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "commit_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=30&since=2021-01-01T00%%3A00%%3A00Z&until=",
			url.PathEscape(owner+"/"+repo1)), createGitLabHandlerForUnknownUrl)
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
		Hash:          "ed899a2f4b50b4370feeea94676502b42383c746",
		AuthorName:    "Example User",
		CommitterName: "Administrator",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
		Timestamp:     1348131022,
		Message:       "Replace sanitize with escape once",
		ParentHashes:  []string{"6104942438c14ec7bd21c6cd5bd995272b3faff6"},
		AuthorEmail:   "user@example.com",
	}, result[0])
	assert.Equal(t, CommitInfo{
		Hash:          "6104942438c14ec7bd21c6cd5bd995272b3faff6",
		AuthorName:    "randx",
		CommitterName: "ExampleName",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
		Timestamp:     1348131022,
		Message:       "Sanitize for network graph",
		ParentHashes:  []string{"ae1d9fb46aa2b07ee9836d49862ec4e2c46fbbba"},
		AuthorEmail:   "user@example.com",
	}, result[1])
}

func TestGitLabClient_GetLatestCommitNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
		"message": "404 Project Not Found"
	}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=50&ref_name=master",
			url.PathEscape(owner+"/"+repo1)), http.StatusNotFound, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404 Project Not Found")
	assert.Empty(t, result)
}

func TestGitLabClient_GetLatestCommitUnknownBranch(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, []byte("[]"),
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=50&ref_name=unknown",
			url.PathEscape(owner+"/"+repo1)), http.StatusOK, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "unknown")

	assert.Error(t, err)
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

	assert.NoError(t, err)
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

	assert.NoError(t, err)
}

func TestGitLabClient_GetRepositoryInfo(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "repository_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, response,
		"/api/v4/projects/diaspora%2Fdiaspora-project-site", http.StatusOK, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetRepositoryInfo(ctx, "diaspora", "diaspora-project-site")
	assert.NoError(t, err)
	assert.Equal(t,
		RepositoryInfo{
			RepositoryVisibility: Private,
			CloneInfo: CloneInfo{
				HTTP: "https://example.com/diaspora/diaspora-project-site.git",
				SSH:  "git@example.com:diaspora/diaspora-project-site.git"},
		},
		result,
	)
}

func TestGitLabClient_GetCommitBySha(t *testing.T) {
	ctx := context.Background()
	sha := "ff4a54b88fbd387ac4d9e8cdeb54b049978e450a"
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "commit_single_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits/%s",
			url.PathEscape(owner+"/"+repo1), sha), createGitLabHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          sha,
		AuthorName:    "Example User",
		CommitterName: "Administrator",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ff4a54b88fbd387ac4d9e8cdeb54b049978e450a",
		Timestamp:     1636383388,
		Message:       "Initial commit",
		ParentHashes:  []string{"667fb1d7f3854da3ee036ba3ad711c87c8b37fbd"},
		AuthorEmail:   "user@example.com",
	}, result)
}

func TestGitLabClient_GetCommitByShaNotFound(t *testing.T) {
	ctx := context.Background()
	sha := "ff4a54b88fbd387ac4d9e8cdeb54b049978e450b"

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, nil,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits/%s",
			url.PathEscape(owner+"/"+repo1), sha), http.StatusNotFound, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)

	assert.Error(t, err)
	assert.ErrorIs(t, err, gitlab.ErrNotFound)
	assert.Empty(t, result)
}

func TestGitLabClient_getGitLabProjectVisibility(t *testing.T) {
	assert.Equal(t, Public, getGitLabProjectVisibility(&gitlab.Project{Visibility: gitlab.PublicVisibility}))
	assert.Equal(t, Internal, getGitLabProjectVisibility(&gitlab.Project{Visibility: gitlab.InternalVisibility}))
	assert.Equal(t, Private, getGitLabProjectVisibility(&gitlab.Project{Visibility: gitlab.PrivateVisibility}))
}

func TestGitlabClient_getGitlabCommitState(t *testing.T) {
	assert.Equal(t, "success", getGitLabCommitState(Pass))
	assert.Equal(t, "failed", getGitLabCommitState(Fail))
	assert.Equal(t, "failed", getGitLabCommitState(Error))
	assert.Equal(t, "running", getGitLabCommitState(InProgress))
	assert.Equal(t, "", getGitLabCommitState(5))
}

func TestGitlabClient_CreateLabel(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.Label{},
		fmt.Sprintf("/api/v4/projects/%s/labels", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	err := client.CreateLabel(ctx, owner, repo1, LabelInfo{
		Name:        labelName,
		Description: "label-description",
		Color:       "001122",
	})
	assert.NoError(t, err)
}

func TestGitlabClient_GetLabel(t *testing.T) {
	ctx := context.Background()
	expectedLabel := gitlab.Label{Name: labelName, Description: "label-description", Color: "001122"}
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []gitlab.Label{expectedLabel},
		fmt.Sprintf("/api/v4/projects/%s/labels", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	labelInfo, err := client.GetLabel(ctx, owner, repo1, labelName)
	assert.NoError(t, err)
	assert.Equal(t, labelInfo.Name, expectedLabel.Name)
	assert.Equal(t, labelInfo.Description, expectedLabel.Description)
	assert.Equal(t, labelInfo.Color, expectedLabel.Color)

	labelInfo, err = client.GetLabel(ctx, owner, repo1, "not-existed")
	assert.NoError(t, err)
	assert.Nil(t, labelInfo)
}

func TestGitlabClient_ListPullRequestLabels(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, &gitlab.MergeRequest{Labels: gitlab.Labels{labelName}},
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/1", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	labels, err := client.ListPullRequestLabels(ctx, owner, repo1, 1)
	assert.NoError(t, err)
	assert.Len(t, labels, 1)
	assert.Equal(t, labelName, labels[0])
}

func TestGitlabClient_UnlabelPullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, nil,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/1", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	err := client.UnlabelPullRequest(ctx, owner, repo1, labelName, 1)
	assert.NoError(t, err)
}

func TestGitlabClient_UploadCodeScanning(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, true, "", "unsupportedTest", createGitLabHandler)
	defer cleanUp()
	_, err := client.UploadCodeScanning(ctx, owner, repo1, "", "1")
	assert.Error(t, err)
}

func TestGitlabClient_GetRepositoryEnvironmentInfo(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, true, "", "unsupportedTest", createGitLabHandler)
	defer cleanUp()

	_, err := client.GetRepositoryEnvironmentInfo(ctx, owner, repo1, envName)
	assert.ErrorIs(t, err, errGitLabGetRepoEnvironmentInfoNotSupported)
}

func TestGitLabClient_DeletePullRequestReviewComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, "",
		"", createGitLabHandlerWithoutExpectedURI)
	defer cleanUp()
	err := client.DeletePullRequestReviewComments(ctx, owner, "", 1, CommentInfo{})
	assert.Error(t, err)
	err = client.DeletePullRequestReviewComments(ctx, owner, "test", 1, CommentInfo{})
	assert.Error(t, err)
	err = client.DeletePullRequestReviewComments(ctx, owner, repo1, 1, []CommentInfo{{ThreadID: "ab22", ID: 2}, {ThreadID: "ba22", ID: 3}}...)
	assert.NoError(t, err)
}

func TestGitLabClient_DeletePullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, "",
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/1/notes/1", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()
	err := client.DeletePullRequestComment(ctx, owner, repo1, 1, 1)
	assert.NoError(t, err)
}

func TestGitLabClient_GetModifiedFiles(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "compare_commits.json"))
		assert.NoError(t, err)

		client, cleanUp := createServerAndClient(
			t,
			vcsutils.GitLab,
			true,
			response,
			fmt.Sprintf("/api/v4/projects/%s/repository/compare?from=sha-1&to=sha-2", url.PathEscape(owner+"/"+repo1)),
			createGitLabHandler,
		)
		defer cleanUp()

		fileNames, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		assert.NoError(t, err)
		assert.Equal(t, []string{
			"doc/user/project/integrations/gitlab_slack_application.md",
			"doc/user/project/integrations/slack.md",
			"doc/user/project/integrations/slack_slash_commands.md",
			"doc/user/project/integrations/slack_slash_commands_2.md",
		}, fileNames)
	})

	t.Run("validation fails", func(t *testing.T) {
		client := GitLabClient{}
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
			vcsutils.GitLab,
			true,
			nil,
			"/api/v4/projects/jfrog%2Frepo-1/repository/compare?from=sha-1&to=sha-2",
			http.StatusInternalServerError,
			createGitLabHandler,
		)
		defer cleanUp()
		_, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api/v4/projects/jfrog/repo-1/repository/compare: 500 failed to parse unexpected error type: <nil>")
	})
}

func createGitLabHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/v4/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Equal(t, expectedURI, r.RequestURI)
		assert.Equal(t, token, r.Header.Get("Private-Token"))
	}
}

// Similar to createGitLabHandler but without checking if the expectedURI is equal to the request URI, only if it contained in the request URI.
func createGitLabHandlerForUnknownUrl(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/v4/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Contains(t, r.RequestURI, expectedURI)
		assert.Equal(t, token, r.Header.Get("Private-Token"))
	}
}

func createGitLabHandlerWithoutExpectedURI(t *testing.T, _ string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/v4/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Equal(t, token, r.Header.Get("Private-Token"))
	}
}

func createAddPullRequestReviewCommentGitLabHandler(t *testing.T, _ string, _ []byte, _ int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/api/v4/projects/jfrog%2Frepo-1/merge_requests/7/versions":
			versionsDiff, err := os.ReadFile(filepath.Join("testdata", "gitlab", "merge_request_diff_versions.json"))
			assert.NoError(t, err)
			_, err = w.Write(versionsDiff)
			assert.NoError(t, err)
			assert.Equal(t, token, r.Header.Get("Private-Token"))
		case "/api/v4/projects/jfrog%2Frepo-1/merge_requests/7/diffs":
			mergeRequestChanges, err := os.ReadFile(filepath.Join("testdata", "gitlab", "merge_request_changes.json"))
			assert.NoError(t, err)
			_, err = w.Write(mergeRequestChanges)
			assert.NoError(t, err)
			assert.Equal(t, token, r.Header.Get("Private-Token"))
		case "/api/v4/projects/jfrog%2Frepo-1/merge_requests/7/discussions":
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			if strings.Contains(string(body), "old_path") {
				w.WriteHeader(http.StatusNotFound)
			}
			newMergeRequestThreadResponse, err := os.ReadFile(filepath.Join("testdata", "gitlab", "new_merge_request_thread.json"))
			assert.NoError(t, err)
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(newMergeRequestThreadResponse)
			assert.NoError(t, err)
		}
	}
}

func createDownloadRepositoryGitLabHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/v4/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.RequestURI == "/api/v4/projects/jfrog%2Frepo-1" {
			repositoryResponse, err := os.ReadFile(filepath.Join("testdata", "gitlab", "repository_response.json"))
			assert.NoError(t, err)
			_, err = w.Write(repositoryResponse)
			assert.NoError(t, err)
			return
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Equal(t, expectedURI, r.RequestURI)
		assert.Equal(t, token, r.Header.Get("Private-Token"))
	}
}

func createGitLabWithPaginationHandler(t *testing.T, _ string, response []byte, _ []byte, expectedStatusCode int, expectedHttpMethod string) http.HandlerFunc {
	var repos []gitlab.Project
	err := json.Unmarshal(response, &repos)
	assert.NoError(t, err)
	const (
		defaultPerPage = 20
		xTotalPages    = "X-Total-Pages"
		qPage          = "page"
		qPerPage       = "per_page"
		qMembership    = "membership"
	)
	count := len(repos)

	return func(writer http.ResponseWriter, request *http.Request) {
		if request.RequestURI == "/api/v4/" {
			writer.WriteHeader(http.StatusOK)
			return
		}

		assert.Equal(t, expectedHttpMethod, request.Method)
		assert.Equal(t, token, request.Header.Get("Private-Token"))

		pageSize := defaultPerPage
		page := 1
		uri, err := url.Parse(request.RequestURI)
		assert.NoError(t, err)

		if uri.Query().Get(qMembership) != "true" {
			assert.Fail(t, "'membership=true' expected in the request uri", "actual 'membership=%v'", uri.Query().Get(qMembership))
			return
		}
		if uri.Query().Has(qPerPage) {
			pageSize, err = strconv.Atoi(uri.Query().Get(qPerPage))
			assert.NoError(t, err)
		}
		if uri.Query().Has(qPage) {
			page, err = strconv.Atoi(uri.Query().Get(qPage))
			assert.NoError(t, err)
			if page <= 0 {
				page = 1
			}
		}

		lastPage := int(math.Ceil(float64(count) / float64(pageSize)))
		writer.Header().Add(xTotalPages, strconv.Itoa(lastPage))
		writer.WriteHeader(expectedStatusCode)

		var pageItems []gitlab.Project
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

func createGitLabWithBodyHandler(t *testing.T, expectedURI string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHTTPMethod string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.RequestURI == "/api/v4/" {
			writer.WriteHeader(http.StatusOK)
			return
		}
		assert.Equal(t, expectedHTTPMethod, request.Method)
		assert.Equal(t, expectedURI, request.RequestURI)
		assert.Equal(t, token, request.Header.Get("Private-Token"))

		b, err := io.ReadAll(request.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedRequestBody, b)

		writer.WriteHeader(expectedStatusCode)
		assert.NoError(t, err)
		_, err = writer.Write(response)
		assert.NoError(t, err)
	}
}
func TestGitLabClient_TestGetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "5fbf81b31ff7a3b06bd362d1891e2f01bdb2be69"
	t.Run("Empty response", func(t *testing.T) {
		client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []CommitStatusInfo{},
			fmt.Sprintf("/api/v4/projects/%s/repository/commits/%s/statuses", repo1, ref),
			createGitLabHandler)
		defer cleanUp()
		_, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.NoError(t, err)
	})
	t.Run("Valid response", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "commits_statuses.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
			fmt.Sprintf("/api/v4/projects/%s/repository/commits/%s/statuses", repo1, ref),
			createGitLabHandler)
		defer cleanUp()
		commitStatuses, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Len(t, commitStatuses, 3)
		assert.Equal(t, Pass, commitStatuses[0].State)
		assert.Equal(t, InProgress, commitStatuses[1].State)
		assert.Equal(t, Fail, commitStatuses[2].State)
		assert.NoError(t, err)
	})
	t.Run("Invalid response format", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "github", "commits_statuses_bad_json.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
			fmt.Sprintf("/api/v4/projects/%s/repository/commits/%s/statuses", repo1, ref),
			createGitLabHandler)
		defer cleanUp()
		_, err = client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Error(t, err)
	})
	t.Run("Bad client", func(t *testing.T) {
		client := createBadGitHubClient(t)
		_, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Error(t, err)
	})
}

func TestGitLabClient_getProjectOwnerByID(t *testing.T) {
	projectID := 47457684

	// Successful response
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "get_project_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%d", projectID), createGitLabHandler)
	defer cleanUp()

	glClient, ok := client.(*GitLabClient)
	assert.True(t, ok)
	projectOwner, err := glClient.getProjectOwnerByID(projectID)
	assert.NoError(t, err)
	assert.Equal(t, "test", projectOwner)

	badClient, badClientCleanUp :=
		createServerAndClient(t, vcsutils.GitLab, false, nil, fmt.Sprintf("/api/v4/projects/%d", projectID), createGitLabHandler)
	defer badClientCleanUp()
	badGlClient, ok := badClient.(*GitLabClient)
	assert.True(t, ok)
	projectOwner, err = badGlClient.getProjectOwnerByID(projectID)
	assert.Error(t, err)
	assert.NotEqual(t, "test", projectOwner)
}
