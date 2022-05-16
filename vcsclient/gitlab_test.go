package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
	id := rand.Int()
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
	id := rand.Int()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks/%d", url.PathEscape(owner+"/"+repo1), id), createGitLabHandler)
	defer cleanUp()

	err := client.UpdateWebhook(ctx, owner, repo1, branch1, "https://jfrog.com", token, strconv.Itoa(id),
		vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected)
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

func TestGitLabClient_AddPullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, &gitlab.MergeRequest{}, fmt.Sprintf("/api/v4/projects/%s/merge_requests/1/notes", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	err := client.AddPullRequestComment(ctx, owner, repo1, "Comment content", 1)
	assert.NoError(t, err)
}

func TestGitLabClient_ListPullRequestComments(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "pull_request_comments_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/merge_requests/1/notes", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	result, err := client.ListPullRequestComments(ctx, owner, repo1, 1)
	require.NoError(t, err)
	expectedCreated, err := time.Parse(time.RFC3339, "2013-10-02T09:56:03Z")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, CommentInfo{
		ID:      305,
		Content: "Text of the comment\r\n",
		Created: expectedCreated,
	}, result[1])
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

func TestGitLabClient_GetLatestCommitUnknownBranch(t *testing.T) {
	ctx := context.Background()

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, []byte("[]"),
		fmt.Sprintf("/api/v4/projects/%s/repository/commits?page=1&per_page=1&ref_name=unknown",
			url.PathEscape(owner+"/"+repo1)), http.StatusOK, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "unknown")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404 Not Found")
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

func TestGitLabClient_GetRepositoryInfo(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "gitlab", "repository_response.json"))
	require.NoError(t, err)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, response,
		"/api/v4/projects/diaspora%2Fdiaspora-project-site", http.StatusOK, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetRepositoryInfo(ctx, "diaspora", "diaspora-project-site")
	require.NoError(t, err)
	require.Equal(t,
		RepositoryInfo{CloneInfo: CloneInfo{
			HTTP: "http://example.com/diaspora/diaspora-project-site.git",
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

	require.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          sha,
		AuthorName:    "Example User",
		CommitterName: "Administrator",
		Url:           "https://gitlab.example.com/thedude/gitlab-foss/-/commit/ff4a54b88fbd387ac4d9e8cdeb54b049978e450a",
		Timestamp:     1636383388,
		Message:       "Initial commit",
		ParentHashes:  []string{"667fb1d7f3854da3ee036ba3ad711c87c8b37fbd"},
	}, result)
}

func TestGitLabClient_GetCommitByShaNotFound(t *testing.T) {
	ctx := context.Background()
	sha := "ff4a54b88fbd387ac4d9e8cdeb54b049978e450b"
	response := []byte(`{
		"message": "404 Commit Not Found"
	}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.GitLab, false, response,
		fmt.Sprintf("/api/v4/projects/%s/repository/commits/%s",
			url.PathEscape(owner+"/"+repo1), sha), http.StatusNotFound, createGitLabHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404 Commit Not Found")
	assert.Empty(t, result)
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
		require.NoError(t, err)
		assert.Equal(t, expectedRequestBody, b)

		writer.WriteHeader(expectedStatusCode)
		assert.NoError(t, err)
		_, err = writer.Write(response)
		assert.NoError(t, err)
	}
}
