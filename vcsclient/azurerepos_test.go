package vcsclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAzureRepos_Connection(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "", createAzureReposHandler)
	defer cleanUp()
	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestAzureRepos_ListRepositories(t *testing.T) {
	type ListRepositoryResponse struct {
		Value []git.GitRepository
		Count int
	}
	testRepos := []string{"test_repo_1", "test_repo_2"}
	res := ListRepositoryResponse{
		Value: []git.GitRepository{{Name: &testRepos[0]}, {Name: &testRepos[1]}},
		Count: 2,
	}
	jsonRes, err := json.Marshal(res)
	assert.NoError(t, err)
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "listRepositories", createAzureReposHandler)
	defer cleanUp()
	reposMap, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	for _, repos := range reposMap {
		assert.ElementsMatch(t, repos, testRepos)
	}
}

func TestAzureRepos_TestListBranches(t *testing.T) {
	type ListBranchesResponse struct {
		Value []git.GitBranchStats
		Count int
	}
	testBranches := []string{"test_branch_1", "test_branch_2"}
	res := ListBranchesResponse{
		Value: []git.GitBranchStats{{Name: &testBranches[0]}, {Name: &testBranches[1]}},
		Count: 2,
	}
	jsonRes, err := json.Marshal(res)
	assert.NoError(t, err)
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "listBranches", createAzureReposHandler)
	defer cleanUp()
	resp, err := client.ListBranches(ctx, "", repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, testBranches, resp)
}

func TestAzureRepos_TestDownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	repoFile, err := os.ReadFile(filepath.Join("testdata", "azurerepos", "hello_world.zip"))
	require.NoError(t, err)

	client, cleanUp := createServerAndClient(
		t,
		vcsutils.AzureRepos,
		true,
		repoFile,
		fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items/items?path=/&[…]ptor[version]=%s&$format=zip",
			username,
			"",
			repo1,
			branch1),
		createAzureReposHandler)
	defer cleanUp()
	err = client.DownloadRepository(ctx, "", repo1, branch1, dir)
	require.NoError(t, err)

	badClient, cleanUp := createBadAzureReposClient(t, repoFile)
	defer cleanUp()
	err = badClient.DownloadRepository(ctx, owner, repo1, branch1, dir)
	assert.Error(t, err)
}

func TestAzureRepos_TestCreatePullRequest(t *testing.T) {
	type CreatePullRequestResponse struct {
		Value git.GitPullRequest
		Count int
	}
	helloWorld := "hello world"
	res := CreatePullRequestResponse{
		Value: git.GitPullRequest{
			Repository:    &git.GitRepository{Name: &repo1},
			SourceRefName: &branch1,
			TargetRefName: &branch2,
			Title:         &helloWorld,
			Description:   &helloWorld,
		},
		Count: 1,
	}
	jsonRes, err := json.Marshal(res)
	assert.NoError(t, err)
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "createPullRequest", createAzureReposHandler)
	defer cleanUp()
	err = client.CreatePullRequest(ctx, "", repo1, branch1, branch2, "Hello World", "Hello World")
	assert.NoError(t, err)
}

func TestAzureRepos_TestAddPullRequestComment(t *testing.T) {
	type AddPullRequestCommentResponse struct {
		Value git.GitPullRequestCommentThread
		Count int
	}
	id := 123
	res := AddPullRequestCommentResponse{
		Value: git.GitPullRequestCommentThread{Id: &id},
		Count: 1,
	}
	jsonRes, err := json.Marshal(res)
	assert.NoError(t, err)
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "addPullRequestComment", createAzureReposHandler)
	defer cleanUp()
	err = client.AddPullRequestComment(ctx, "", repo1, "test", 2)
	assert.NoError(t, err)
}

func TestAzureRepos_TestListOpenPullRequests(t *testing.T) {
	type ListOpenPullRequestsResponse struct {
		Value []git.GitPullRequest
		Count int
	}
	pullRequestId := 1
	res := ListOpenPullRequestsResponse{
		Value: []git.GitPullRequest{
			{
				PullRequestId: &pullRequestId,
				Repository:    &git.GitRepository{Name: &repo1},
				SourceRefName: &branch1,
				TargetRefName: &branch2,
			},
		},
		Count: 1,
	}
	jsonRes, err := json.Marshal(res)
	assert.NoError(t, err)
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "listOpenPullRequests", createAzureReposHandler)
	defer cleanUp()
	pullRequestsInfo, err := client.ListOpenPullRequests(ctx, "", repo1)
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(pullRequestsInfo, []PullRequestInfo{{ID: 1, Source: BranchInfo{Name: branch1, Repository: repo1}, Target: BranchInfo{Name: branch2, Repository: repo1}}}))
}

func TestAzureRepos_TestGetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "azurerepos", "commits.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, response, "getLatestCommit", createAzureReposHandler)
	defer cleanUp()

	commit, err := client.GetLatestCommit(ctx, "", repo1, branch1)
	assert.Equal(t, commit, CommitInfo{
		Hash:          "86d6919952702f9ab03bc95b45687f145a663de0",
		AuthorName:    "Test User",
		CommitterName: "Test User",
		Url:           "https://dev.azure.com/testuser/0b8072c4-ad86-4edb-a8f2-06dbc07e3e2d/_apis/git/repositories/94c1dba8-d9d9-4600-94b4-1a51acb43220/commits/86d6919952702f9ab03bc95b45687f145a663de0",
		Timestamp:     1667812601,
		Message:       "Updated package.json",
	})
	assert.NoError(t, err)
}

func createAzureReposHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base64Token := base64.StdEncoding.EncodeToString([]byte(":" + token))
		assert.Equal(t, "Basic "+base64Token, r.Header.Get("Authorization"))
		if r.RequestURI == "/frogger/_apis" {
			jsonVal, err := os.ReadFile(filepath.Join("./", "testdata", "azurerepos", "resourcesResponse.json"))
			assert.NoError(t, err)
			w.Write(jsonVal)
			return
		} else if r.RequestURI == "/frogger/_apis/ResourceAreas" {
			jsonVal := `{"value": [],"count": 0}`
			w.Write([]byte(jsonVal))
			return
		}

		if !strings.Contains(expectedURI, "bad^endpoint") {
			assert.Contains(t, r.RequestURI, expectedURI)
			w.WriteHeader(expectedStatusCode)
			_, err := w.Write(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(404)
		return
	}
}

func createBadAzureReposClient(t *testing.T, repoFile []byte) (VcsClient, func()) {
	client, cleanUp := createServerAndClient(
		t,
		vcsutils.AzureRepos,
		true,
		repoFile,
		fmt.Sprintf("bad^endpoint/%s/_apis/git/repositories/%s/items/items?path=/&[…]ptor[version]=%s&$format=zip",
			"",
			repo1,
			branch1),
		createAzureReposHandler)
	return client, cleanUp
}