package vcsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
		"/rest/api/1.0/admin/users?limit=1", createBitbucketServerHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).TestConnection(ctx)
	assert.Error(t, err)
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
	assert.Equal(t, map[string][]string{"~" + strings.ToUpper(username): {repo1}, username: {repo2}}, actualRepositories)

	_, err = createBadBitbucketServerClient(t).ListRepositories(ctx)
	assert.Error(t, err)
}

func TestBitbucketServer_ListBranches(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucketv1.Branch{
		"values": {{ID: branch1}, {ID: branch2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, mockResponse, "/rest/api/1.0/projects/jfrog/repos/repo-1/branches?start=0", createBitbucketServerHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})

	_, err = createBadBitbucketServerClient(t).ListBranches(ctx, owner, repo1)
	assert.Error(t, err)
}

func TestBitbucketServer_CreateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31() // #nosec G404
	mockResponse := bitbucketv1.Webhook{ID: int(id)}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, mockResponse, "/rest/api/1.0/projects/jfrog/repos/repo-1/webhooks", createBitbucketServerHandler)
	defer cleanUp()

	actualID, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything",
		vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, strconv.Itoa(int(id)), actualID)

	_, _, err = createBadBitbucketServerClient(t).CreateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", vcsutils.Push)
	assert.Error(t, err)
}

func TestBitbucketServer_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31() // #nosec G404
	stringID := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/rest/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringID), createBitbucketServerHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", token, stringID,
		vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected, vcsutils.TagPushed, vcsutils.TagRemoved)
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).UpdateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", token, stringID, vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected)
	assert.Error(t, err)
}

func TestBitbucketServer_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id := rand.Int31() // #nosec G404
	stringID := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/rest/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringID), createBitbucketServerHandler)
	defer cleanUp()

	err := client.DeleteWebhook(ctx, owner, repo1, stringID)
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).DeleteWebhook(ctx, owner, repo1, stringID)
	assert.Error(t, err)
}

func TestBitbucketServer_ListPullRequestReviews(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/reviewers", owner, repo1, 1), createBitbucketServerHandler)
	defer cleanUp()
	_, err := client.ListPullRequestReviews(ctx, owner, repo1, 1)
	assert.ErrorIs(t, err, errBitbucketListListPullRequestReviewsNotSupported)
}

func TestBitbucketServer_ListPullCommits(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/commits", owner, repo1, 1), createBitbucketServerHandler)
	defer cleanUp()
	_, err := client.ListPullRequestCommits(ctx, owner, repo1, 1)
	assert.ErrorIs(t, err, errBitbucketListPullRequestCommitsNotSupported)
}

func TestBitbucketServer_ListPullRequestsAssociatedWithCommit(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits/%s/pull-requests", owner, repo1, "commitSHA"), createBitbucketServerHandler)
	defer cleanUp()
	_, err := client.ListPullRequestsAssociatedWithCommit(ctx, owner, repo1, "commitSHA")
	assert.ErrorIs(t, err, errBitbucketListPullRequestAssociatedCommitsNotSupported)
}

func TestBitbucketServer_SetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/rest/build-status/1.0/commits/%s", ref), createBitbucketServerHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Fail, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).SetCommitStatus(ctx, Fail, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.Error(t, err)
}

func TestBitbucketServer_DownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer func() { assert.NoError(t, vcsutils.RemoveTempDir(dir)) }()

	repoFile, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "hello-world-main.tar.gz"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, repoFile,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/archive?format=tgz", owner, repo1), createBitbucketServerDownloadRepositoryHandler)
	defer cleanUp()
	err = client.DownloadRepository(ctx, owner, repo1, "", dir)
	assert.NoError(t, err)

	readmeFile, err := os.OpenFile(filepath.Join(dir, "README.md"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	assert.NoError(t, err)
	defer func() {
		err = errors.Join(err, readmeFile.Close())
	}()
	err = createBadBitbucketServerClient(t).DownloadRepository(ctx, "ssa", "solr-system", "master", dir)
	assert.Error(t, err)
}

func TestBitbucketServer_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, "/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests", createBitbucketServerHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.Error(t, err)
}

func TestBitbucketServer_UpdatePullRequest(t *testing.T) {
	prId := 4
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, fmt.Sprintf("/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests/%v", prId), createBitbucketServerHandler)
	defer cleanUp()

	err := client.UpdatePullRequest(ctx, owner, repo1, "PR title", "PR body", "", prId, vcsutils.Open)
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).UpdatePullRequest(ctx, owner, repo1, "PR title", "PR body", "", prId, vcsutils.Open)
	assert.Error(t, err)
}

func TestBitbucketServer_AddPullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, "/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests/1/comments", createBitbucketServerHandler)
	defer cleanUp()

	err := client.AddPullRequestComment(ctx, owner, repo1, "Comment content", 1)
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).AddPullRequestComment(ctx, owner, repo1, "Comment content", 1)
	assert.Error(t, err)
}

func TestBitbucketServer_AddPullRequestReviewComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, "/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests/1/comments", createBitbucketServerHandler)
	defer cleanUp()

	err := client.AddPullRequestReviewComments(ctx, owner, repo1, 1, PullRequestComment{CommentInfo: CommentInfo{Content: "Comment content"}, PullRequestDiff: PullRequestDiff{OriginalStartLine: 7, NewStartLine: 7}})
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).AddPullRequestReviewComments(ctx, owner, repo1, 1, PullRequestComment{CommentInfo: CommentInfo{Content: "Comment content"}, PullRequestDiff: PullRequestDiff{OriginalStartLine: 7, NewStartLine: 7}})
	assert.Error(t, err)
}

func TestBitbucketServer_ListOpenPullRequests(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "pull_requests_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests?start=0", owner, repo1), createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.ListOpenPullRequests(ctx, owner, repo1)
	forkedOwner := "jfrogForked"
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.EqualValues(t, PullRequestInfo{
		ID:     101,
		Title:  "Talking Nerdy",
		Author: "tom",
		Source: BranchInfo{Name: "feature-ABC-123", Repository: repo1, Owner: forkedOwner},
		Target: BranchInfo{Name: "master", Repository: repo1, Owner: owner},
		URL:    "https://link/to/pullrequest",
	}, result[0])

	// With body:
	result, err = client.ListOpenPullRequestsWithBody(ctx, owner, repo1)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.EqualValues(t, PullRequestInfo{
		ID:     101,
		Title:  "Talking Nerdy",
		Author: "tom",
		Body:   "hello world",
		Source: BranchInfo{Name: "feature-ABC-123", Repository: repo1, Owner: forkedOwner},
		Target: BranchInfo{Name: "master", Repository: repo1, Owner: owner},
		URL:    "https://link/to/pullrequest",
	}, result[0])
}

func TestBitbucketServerClient_GetPullRequest(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "get_pull_request_response.json"))
	assert.NoError(t, err)
	pullRequestId := 6

	// Successful
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d", owner, repo1, pullRequestId), createBitbucketServerHandler)
	defer cleanUp()
	result, err := client.GetPullRequestByID(ctx, owner, repo1, pullRequestId)
	assert.NoError(t, err)
	assert.EqualValues(t, PullRequestInfo{
		ID:     int64(pullRequestId),
		Title:  "New vul 2",
		Author: "owner",
		Source: BranchInfo{Name: "new_vul_2", Repository: "repoName", Owner: "~fromOwner"},
		Target: BranchInfo{Name: "master", Repository: "repoName", Owner: owner},
		URL:    "https://git.bbServerHost.info/users/owner/repos/repoName/pull-requests/6",
	}, result)

	// Failed owner extraction
	response, err = os.ReadFile(filepath.Join("testdata", "bitbucketserver", "get_pull_request_response_nil.json"))
	assert.NoError(t, err)
	ownerClient, ownerCleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d", owner, repo1, pullRequestId), createBitbucketServerHandler)
	defer ownerCleanUp()
	_, err = ownerClient.GetPullRequestByID(ctx, owner, repo1, pullRequestId)
	assert.Error(t, err)

	// Bad response
	badClient, badClientCleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, "{",
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d", owner, repo1, pullRequestId), createBitbucketServerHandler)
	defer badClientCleanUp()
	_, err2 := badClient.GetPullRequestByID(ctx, owner, repo1, pullRequestId)
	assert.Error(t, err2)

	// Bad Client
	_, err = createBadBitbucketServerClient(t).GetPullRequestByID(ctx, owner, repo1, pullRequestId)
	assert.Error(t, err)
}

func TestBitbucketServer_ListPullRequestReviewComments(t *testing.T) {
	TestBitbucketServer_ListPullRequestComments(t)
}

func TestBitbucketServer_ListPullRequestComments(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "pull_request_comments_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/1/activities?start=0", owner, repo1), createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.ListPullRequestComments(ctx, owner, repo1, 1)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, CommentInfo{
		ID:      1,
		Content: "A measured reply.",
		Created: time.Unix(1548720847370, 0),
		Version: 1,
	}, result[0])
}

func TestBitbucketServer_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "commit_list_response.json"))
	assert.NoError(t, err)

	// limit=1 appears twice because it is added twice by: github.com/gfleury/go-bitbucket-v1@v0.0.0-20210826163055-dff2223adeac/default_api.go:3848
	client, serverUrl, cleanUp := createServerWithUrlAndClientReturningStatus(t, vcsutils.BitbucketServer, false,
		response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits?limit=50&limit=50&until=master", owner, repo1),
		http.StatusOK, createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	assert.NoError(t, err)
	expectedUrl := fmt.Sprintf("%s/projects/jfrog/repos/repo-1"+
		"/commits/def0123abcdef4567abcdef8987abcdef6543abc", serverUrl)
	assert.Equal(t, CommitInfo{
		Hash:          "def0123abcdef4567abcdef8987abcdef6543abc",
		AuthorName:    "charlie",
		CommitterName: "mark",
		Url:           expectedUrl,
		Timestamp:     1548720847,
		Message:       "More work on feature 1",
		ParentHashes:  []string{"abcdef0123abcdef4567abcdef8987abcdef6543", "qwerty0123abcdef4567abcdef8987abcdef6543"},
		AuthorEmail:   "charlie@example.com",
	}, result)

	_, err = createBadBitbucketServerClient(t).GetLatestCommit(ctx, owner, repo1, "master")
	assert.Error(t, err)
}

func TestBitbucketServer_GetCommits(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "commit_list_response.json"))
	assert.NoError(t, err)

	client, serverUrl, cleanUp := createServerWithUrlAndClientReturningStatus(t, vcsutils.BitbucketServer, false,
		response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits?limit=50&limit=50&until=master", owner, repo1),
		http.StatusOK, createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetCommits(ctx, owner, repo1, "master")

	assert.NoError(t, err)
	expectedUrl := fmt.Sprintf("%s/projects/jfrog/repos/repo-1"+
		"/commits/def0123abcdef4567abcdef8987abcdef6543abc", serverUrl)
	assert.Equal(t, CommitInfo{
		Hash:          "def0123abcdef4567abcdef8987abcdef6543abc",
		AuthorName:    "charlie",
		CommitterName: "mark",
		Url:           expectedUrl,
		Timestamp:     1548720847,
		Message:       "More work on feature 1",
		ParentHashes:  []string{"abcdef0123abcdef4567abcdef8987abcdef6543", "qwerty0123abcdef4567abcdef8987abcdef6543"},
		AuthorEmail:   "charlie@example.com",
	}, result[0])
	assert.Equal(t, CommitInfo{
		Hash:          "def0123abcdef4567abcdef8987abcdef6543abc",
		AuthorName:    "marly",
		CommitterName: "marly",
		Url:           expectedUrl,
		Timestamp:     1548720847,
		Message:       "More work on feature 2",
		ParentHashes:  []string{"abcdef0123abcdef4567abcdef8987abcdef6543", "qwerty0123abcdef4567abcdef8987abcdef6543"},
		AuthorEmail:   "marly@example.com",
	}, result[1])

	_, err = createBadBitbucketServerClient(t).GetCommits(ctx, owner, repo1, "master")
	assert.Error(t, err)
}

func TestBitbucketServer_GetCommitsWithQueryOptions(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "commit_list_response.json"))
	assert.NoError(t, err)
	client, serverUrl, cleanUp := createServerWithUrlAndClientReturningStatus(t, vcsutils.BitbucketServer, false,
		response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits?limit=30&limit=30&start=0", owner, repo1),
		http.StatusOK, createBitbucketServerHandler)
	defer cleanUp()

	options := GitCommitsQueryOptions{
		Since: time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
		ListOptions: ListOptions{
			Page:    1,
			PerPage: 30,
		},
	}

	result, err := client.GetCommitsWithQueryOptions(ctx, owner, repo1, options)

	assert.NoError(t, err)
	expectedUrl := fmt.Sprintf("%s/projects/jfrog/repos/repo-1"+
		"/commits/def0123abcdef4567abcdef8987abcdef6543abc", serverUrl)
	assert.Equal(t, CommitInfo{
		Hash:          "def0123abcdef4567abcdef8987abcdef6543abc",
		AuthorName:    "charlie",
		CommitterName: "mark",
		Url:           expectedUrl,
		Timestamp:     1548720847,
		Message:       "More work on feature 1",
		ParentHashes:  []string{"abcdef0123abcdef4567abcdef8987abcdef6543", "qwerty0123abcdef4567abcdef8987abcdef6543"},
		AuthorEmail:   "charlie@example.com",
	}, result[0])
	assert.Equal(t, CommitInfo{
		Hash:          "def0123abcdef4567abcdef8987abcdef6543abc",
		AuthorName:    "marly",
		CommitterName: "marly",
		Url:           expectedUrl,
		Timestamp:     1548720847,
		Message:       "More work on feature 2",
		ParentHashes:  []string{"abcdef0123abcdef4567abcdef8987abcdef6543", "qwerty0123abcdef4567abcdef8987abcdef6543"},
		AuthorEmail:   "marly@example.com",
	}, result[1])

	_, err = createBadBitbucketServerClient(t).GetCommitsWithQueryOptions(ctx, owner, repo1, options)
	assert.Error(t, err)
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
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits?limit=50&limit=50&until=master", owner, repo1),
		http.StatusNotFound, createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Status: 404 Not Found")
	assert.Empty(t, result)
}

func TestBitbucketServer_GetLatestCommitUnknownBranch(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
		"errors": [
			{
				"context": null,
				"exceptionName": "com.atlassian.bitbucket.commit.NoSuchCommitException",
				"message": "Commit 'unknown' does not exist in repository 'test'."
			}
		]
	}`)
	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketServer, false, response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits?limit=50&limit=50&until=unknown", owner, repo1),
		http.StatusNotFound, createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "unknown")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Status: 404 Not Found")
	assert.Empty(t, result)
}

func TestBitbucketServer_AddSshKeyToRepository(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "add_ssh_key_response.json"))
	assert.NoError(t, err)

	expectedBody := []byte(`{"key":{"text":"ssh-rsa AAAA...","label":"My deploy key"},"permission":"REPO_READ"}` + "\n")

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.BitbucketServer, false,
		response, fmt.Sprintf("/keys/1.0/projects/%s/repos/%s/ssh", owner, repo1), http.StatusOK,
		expectedBody, http.MethodPost,
		createBitbucketServerWithBodyHandler)
	defer closeServer()

	err = client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", Read)
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", Read)
	assert.Error(t, err)
}

func TestBitbucketServer_AddSshKeyToRepositoryReadWrite(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "add_ssh_key_response.json"))
	assert.NoError(t, err)

	expectedBody := []byte(`{"key":{"text":"ssh-rsa AAAA...","label":"My deploy key"},"permission":"REPO_WRITE"}` + "\n")

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.BitbucketServer, false,
		response, fmt.Sprintf("/keys/1.0/projects/%s/repos/%s/ssh", owner, repo1), http.StatusOK,
		expectedBody, http.MethodPost,
		createBitbucketServerWithBodyHandler)
	defer closeServer()

	err = client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", ReadWrite)

	assert.NoError(t, err)
}

func TestBitbucketServer_AddSshKeyToRepositoryNotFound(t *testing.T) {
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

	expectedBody := []byte(`{"key":{"text":"ssh-rsa AAAA...","label":"My deploy key"},"permission":"REPO_READ"}` + "\n")

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.BitbucketServer, false,
		response, fmt.Sprintf("/keys/1.0/projects/%s/repos/%s/ssh", "unknown", repo1), http.StatusNotFound,
		expectedBody, http.MethodPost,
		createBitbucketServerWithBodyHandler)
	defer closeServer()

	err := client.AddSshKeyToRepository(ctx, "unknown", repo1, "My deploy key", "ssh-rsa AAAA...", Read)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status: 404 Not Found")
}

func TestBitbucketServer_GetRepositoryInfo(t *testing.T) {
	ctx := context.Background()

	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "repository_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClientReturningStatus(
		t,
		vcsutils.BitbucketServer,
		false,
		response,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s", owner, repo1),
		http.StatusOK,
		createBitbucketServerHandler,
	)
	defer cleanUp()

	t.Run("ok", func(t *testing.T) {
		res, err := client.GetRepositoryInfo(ctx, owner, repo1)
		assert.NoError(t, err)
		assert.Equal(t,
			RepositoryInfo{
				RepositoryVisibility: Public,
				CloneInfo: CloneInfo{
					HTTP: "https://bitbucket.org/jfrog/repo-1.git",
					SSH:  "ssh://git@bitbucket.org:jfrog/repo-1.git",
				},
			},
			res,
		)
	})

	_, err = createBadBitbucketServerClient(t).GetRepositoryInfo(ctx, owner, repo1)
	assert.Error(t, err)
}

func TestBitbucketServer_CreateLabel(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketServer).Build()
	assert.NoError(t, err)

	err = client.CreateLabel(ctx, owner, repo1, LabelInfo{})
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketServer_GetLabel(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketServer).Build()
	assert.NoError(t, err)

	_, err = client.GetLabel(ctx, owner, repo1, labelName)
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketServer_ListPullRequestLabels(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketServer).Build()
	assert.NoError(t, err)

	_, err = client.ListPullRequestLabels(ctx, owner, repo1, 1)
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketServer_UnlabelPullRequest(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketServer).Build()
	assert.NoError(t, err)

	err = client.UnlabelPullRequest(ctx, owner, repo1, labelName, 1)
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketServer_GetRepositoryEnvironmentInfo(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketServer).Build()
	assert.NoError(t, err)

	_, err = client.GetRepositoryEnvironmentInfo(ctx, owner, repo1, envName)
	assert.ErrorIs(t, err, errBitbucketGetRepoEnvironmentInfoNotSupported)
}

func TestBitbucketServer_GetCommitBySha(t *testing.T) {
	ctx := context.Background()
	sha := "abcdef0123abcdef4567abcdef8987abcdef6543"
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "commit_single_response.json"))
	assert.NoError(t, err)

	client, serverUrl, cleanUp := createServerWithUrlAndClientReturningStatus(t, vcsutils.BitbucketServer, false,
		response, fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits/%s", owner, repo1, sha),
		http.StatusOK, createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)

	assert.NoError(t, err)
	expectedUrl := fmt.Sprintf("%s/projects/jfrog/repos/repo-1"+
		"/commits/abcdef0123abcdef4567abcdef8987abcdef6543", serverUrl)
	assert.Equal(t, CommitInfo{
		Hash:          sha,
		AuthorName:    "charlie",
		CommitterName: "mark",
		Url:           expectedUrl,
		Timestamp:     1636089306,
		Message:       "WIP on feature 1",
		ParentHashes:  []string{"bbcdef0123abcdef4567abcdef8987abcdef6543"},
		AuthorEmail:   "charlie@example.com",
	}, result)

	_, err = createBadBitbucketServerClient(t).GetCommitBySha(ctx, owner, repo1, sha)
	assert.Error(t, err)
}

func TestBitbucketServer_GetCommitByShaNotFound(t *testing.T) {
	ctx := context.Background()
	sha := "bbcdef0123abcdef4567abcdef8987abcdef6543"
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
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits/%s", owner, repo1, sha),
		http.StatusNotFound, createBitbucketServerHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Status: 404 Not Found")
	assert.Empty(t, result)
}

func TestBitbucketServer_UploadCodeScanning(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, "", "unsupportedTest", createBitbucketServerHandler)
	defer cleanUp()
	_, err := client.UploadCodeScanning(ctx, owner, repo1, "", "1")
	assert.Error(t, err)
}

func TestBitbucketServer_DownloadFileFromRepo(t *testing.T) {
	ctx := context.Background()
	expectedPayload := []byte("hello world")
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, expectedPayload, "/rest/api/1.0/projects/jfrog/repos/repo-1/raw/hello-world?at=branch-1", createBitbucketServerDownloadFileFromRepositoryHandler)
	defer cleanUp()

	expectedStatusCode := 200
	payload, statusCode, err := client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "hello-world")
	assert.Equal(t, expectedPayload, payload)
	assert.Equal(t, expectedStatusCode, statusCode)
	assert.NoError(t, err)

	client = createBadBitbucketServerClient(t)
	_, _, err = client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "hello-world")
	assert.Error(t, err)

	client, cleanUp = createServerAndClient(t, vcsutils.BitbucketServer, true, expectedPayload, "/rest/api/1.0/projects/jfrog/repos/repo-1/raw/bad-test?at=branch-1", createBitbucketServerDownloadFileFromRepositoryHandler)
	defer cleanUp()
	_, _, err = client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "bad-test")
	assert.Error(t, err)
}

func TestBitbucketServer_getRepositoryVisibility(t *testing.T) {
	assert.Equal(t, Public, getBitbucketServerRepositoryVisibility(true))
	assert.Equal(t, Private, getBitbucketServerRepositoryVisibility(false))
}

func TestBitbucketServerClient_GetModifiedFiles(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "compare_commits.json"))
		assert.NoError(t, err)

		client, closeServer := createBodyHandlingServerAndClient(
			t,
			vcsutils.BitbucketServer,
			false,
			response,
			"/rest/api/1.0/projects/jfrog/repos/repo-1/compare/diff?contextLines=0&from=sha-2&to=sha-1",
			http.StatusOK,
			nil,
			http.MethodGet,
			createBitbucketServerWithBodyHandler,
		)
		defer closeServer()

		actual, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		assert.NoError(t, err)
		assert.Equal(t, []string{"path/to/file.txt", "path/to/other_file.txt", "path/to/other_file2.txt"}, actual)
	})

	t.Run("validation fails", func(t *testing.T) {
		client := BitbucketServerClient{}
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
			vcsutils.BitbucketServer,
			true,
			nil,
			"/rest/api/1.0/projects/jfrog/repos/repo-1/compare/diff?contextLines=0&from=sha-2&to=sha-1",
			http.StatusInternalServerError,
			createBitbucketServerHandler,
		)
		defer cleanUp()
		_, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		assert.EqualError(t, err, "Status: 500 Internal Server Error, Body: null")
	})
}

func createBitbucketServerHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Contains(t, expectedURI, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	}
}

func createBitbucketServerDownloadRepositoryHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/rest/api/1.0/projects/jfrog/repos/repo-1" {
			repositoryResponse, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "repository_response.json"))
			assert.NoError(t, err)
			_, err = w.Write(repositoryResponse)
			assert.NoError(t, err)
		}
		w.WriteHeader(expectedStatusCode)
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Contains(t, expectedURI, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	}
}

func createBitbucketServerListRepositoriesHandler(t *testing.T, _ string, _ []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var responseObj interface{}
		switch r.RequestURI {
		case "/rest/api/1.0/projects?start=0":
			responseObj = map[string][]bitbucketv1.Project{"values": {{Key: username}}}
			w.Header().Add("X-Ausername", username)
		case "/rest/api/1.0/projects/~FROGGER/repos?start=0":
			responseObj = map[string][]bitbucketv1.Repository{"values": {{Slug: repo1}}}
		case "/rest/api/1.0/projects/frogger/repos?start=0":
			responseObj = map[string][]bitbucketv1.Repository{"values": {{Slug: repo2}}}
		default:
			assert.Fail(t, "Unexpected request Uri "+r.RequestURI)
		}
		w.WriteHeader(expectedStatusCode)
		response, err := json.Marshal(responseObj)
		assert.NoError(t, err)
		_, err = w.Write(response)
		assert.NoError(t, err)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	}
}

func createBitbucketServerDownloadFileFromRepositoryHandler(t *testing.T, _ string, expectedResponse []byte, _ int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/rest/api/1.0/projects/jfrog/repos/repo-1/raw/hello-world?at=branch-1" {
			_, err := w.Write(expectedResponse)
			assert.NoError(t, err)
			return
		}
		if r.RequestURI == "/rest/api/1.0/projects/jfrog/repos/repo-1/raw/bad-test?at=branch-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}
}

func createBitbucketServerWithBodyHandler(t *testing.T, expectedURI string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHTTPMethod string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, expectedHTTPMethod, request.Method)
		assert.Equal(t, expectedURI, request.RequestURI)
		assert.Equal(t, "Bearer "+token, request.Header.Get("Authorization"))

		if expectedRequestBody == nil {
			expectedRequestBody = []byte{}
		}
		b, err := io.ReadAll(request.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedRequestBody, b)

		writer.WriteHeader(expectedStatusCode)
		_, err = writer.Write(response)
		assert.NoError(t, err)
	}
}
func TestBitbucketServer_TestGetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	t.Run("Empty response", func(t *testing.T) {
		client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil,
			fmt.Sprintf("/rest/build-status/1.0/commits/%s", ref), createBitbucketServerHandler)
		defer cleanUp()
		_, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.NoError(t, err)
	})
	t.Run("Valid response", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "commits_statuses.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, response,
			fmt.Sprintf("/rest/build-status/1.0/commits/%s", ref), createBitbucketServerHandler)
		defer cleanUp()
		commitStatuses, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.NoError(t, err)
		assert.Len(t, commitStatuses, 3)
		assert.Equal(t, InProgress, commitStatuses[0].State)
		assert.Equal(t, Pass, commitStatuses[1].State)
		assert.Equal(t, Fail, commitStatuses[2].State)
	})
	t.Run("Decode failure", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver", "commits_statuses_bad_decode.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, response,
			fmt.Sprintf("/rest/build-status/1.0/commits/%s", ref), createBitbucketServerHandler)
		defer cleanUp()
		_, err = client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Error(t, err)
	})
	t.Run("bad client", func(t *testing.T) {
		client := createBadBitbucketServerClient(t)
		_, err := client.GetCommitStatuses(ctx, owner, repo1, ref)
		assert.Error(t, err)
	})
}

func TestBitbucketServerClient_DeletePullRequestReviewComments(t *testing.T) {
	ctx := context.Background()
	prId := 4
	commentId := 10
	version := 0
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, fmt.Sprintf("/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests/%v/activities?start=%v", prId, version)+
		fmt.Sprintf("/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests/%v/comments/%v?version=%v", prId, commentId, version), createBitbucketServerHandler)
	defer cleanUp()

	err := client.DeletePullRequestReviewComments(ctx, owner, repo1, prId, CommentInfo{ID: int64(commentId)})
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).DeletePullRequestReviewComments(ctx, owner, repo1, prId, CommentInfo{ID: int64(commentId)})
	assert.Error(t, err)
}

func TestBitbucketServerClient_DeletePullRequestComment(t *testing.T) {
	ctx := context.Background()
	prId := 4
	commentId := 10
	version := 0
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, fmt.Sprintf("/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests/%v/activities?start=%v", prId, version)+
		fmt.Sprintf("/rest/api/1.0/projects/jfrog/repos/repo-1/pull-requests/%v/comments/%v?version=%v", prId, commentId, version), createBitbucketServerHandler)
	defer cleanUp()

	err := client.DeletePullRequestComment(ctx, owner, repo1, prId, commentId)
	assert.NoError(t, err)

	err = createBadBitbucketServerClient(t).DeletePullRequestComment(ctx, owner, repo1, prId, commentId)
	assert.Error(t, err)
}

func createBadBitbucketServerClient(t *testing.T) VcsClient {
	client, err := NewClientBuilder(vcsutils.BitbucketServer).ApiEndpoint("https://bad^endpoint").Build()
	assert.NoError(t, err)
	return client
}

func TestGetCommitsInDateRate(t *testing.T) {
	tests := []struct {
		name     string
		commits  []CommitInfo
		options  GitCommitsQueryOptions
		expected []CommitInfo
	}{
		{
			name: "All commits within range",
			commits: []CommitInfo{
				{Timestamp: 1717396600}, // Mon, 03 Jun 2024 09:56:40 GMT (Within range)
				{Timestamp: 1717396500}, // Mon, 03 Jun 2024 09:55:00 GMT (Within range)
				{Timestamp: 1717396400}, // Mon, 03 Jun 2024 09:53:20 GMT (Within range)
			},
			options: GitCommitsQueryOptions{
				Since: time.Unix(1717396300, 0), // Mon, 03 Jun 2024 09:51:40 GMT (Set since timestamp in seconds)
			},
			expected: []CommitInfo{
				{Timestamp: 1717396600},
				{Timestamp: 1717396500},
				{Timestamp: 1717396400},
			},
		},
		{
			name: "All commits within range or equal",
			commits: []CommitInfo{
				{Timestamp: 1717396600}, // Mon, 03 Jun 2024 09:56:40 GMT (Within range)
				{Timestamp: 1717396500}, // Mon, 03 Jun 2024 09:55:00 GMT (Within range)
				{Timestamp: 1717396400}, // Mon, 03 Jun 2024 09:53:20 GMT (Within range)
			},
			options: GitCommitsQueryOptions{
				Since: time.Unix(1717396400, 0), // Mon, 03 Jun 2024 09:53:20 GMT (Set since timestamp in seconds)
			},
			expected: []CommitInfo{
				{Timestamp: 1717396600},
				{Timestamp: 1717396500},
				{Timestamp: 1717396400},
			},
		},
		{
			name: "No commits within range",
			commits: []CommitInfo{
				{Timestamp: 1717396500}, // Mon, 03 Jun 2024 09:55:00 GMT (Older than range)
				{Timestamp: 1717396400}, // Mon, 03 Jun 2024 09:53:20 GMT (Older than range)
			},
			options: GitCommitsQueryOptions{
				Since: time.Unix(1717396600, 0), // Mon, 03 Jun 2024 09:56:40 GMT (Set since timestamp in seconds)
			},
			expected: []CommitInfo{},
		},
		{
			name: "Partial commits within range",
			commits: []CommitInfo{
				{Timestamp: 1717396600}, // Mon, 03 Jun 2024 09:56:40 GMT (Within range)
				{Timestamp: 1717396500}, // Mon, 03 Jun 2024 09:55:00 GMT (Within range)
				{Timestamp: 1717396400}, // Mon, 03 Jun 2024 09:53:20 GMT (Older than range)
			},
			options: GitCommitsQueryOptions{
				Since: time.Unix(1717396500, 0), // Mon, 03 Jun 2024 09:55:00 GMT (Set since timestamp in seconds)
			},
			expected: []CommitInfo{
				{Timestamp: 1717396600},
				{Timestamp: 1717396500},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCommitsInDateRate(tt.commits, tt.options)
			assert.ElementsMatch(t, result, tt.expected)
		})
	}
}
