package vcsclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

	badClient, badClientCleanup := createBadAzureReposClient(t, []byte{})
	defer badClientCleanup()
	_, err = badClient.ListRepositories(ctx)
	assert.Error(t, err)
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

	badClient, badClientCleanup := createBadAzureReposClient(t, []byte{})
	defer badClientCleanup()
	_, err = badClient.ListBranches(ctx, "", repo1)
	assert.Error(t, err)
}

func TestAzureRepos_TestDownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	repoFile, err := os.ReadFile(filepath.Join("testdata", "azurerepos", "hello_world.zip"))
	require.NoError(t, err)

	downloadURL := fmt.Sprintf("/%s/_apis/git/repositories/%s/items/items?path=/&versionDescriptor[version]=%s&$format=zip",
		"",
		repo1,
		branch1)
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true,
		repoFile, downloadURL, createAzureReposHandler)
	defer cleanUp()
	err = client.DownloadRepository(ctx, "", repo1, branch1, dir)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "README.md"))
	assert.DirExists(t, filepath.Join(dir, ".git"))

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
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "getPullRequests", createAzureReposHandler)
	defer cleanUp()
	err = client.CreatePullRequest(ctx, "", repo1, branch1, branch2, "Hello World", "Hello World")
	assert.NoError(t, err)

	badClient, cleanUp := createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	err = badClient.CreatePullRequest(ctx, "", repo1, branch1, branch2, "Hello World", "Hello World")
	assert.Error(t, err)
}

func TestAzureReposClient_TestUpdatePullRequest(t *testing.T) {
	ctx := context.Background()
	pullRequestId := 1
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, []byte("{}"), "", createAzureReposHandler)
	defer cleanUp()
	err := client.UpdatePullRequest(ctx, owner, repo1, "Hello World", "Hello World", "", pullRequestId, vcsutils.Open)
	assert.NoError(t, err)

	err = client.UpdatePullRequest(ctx, owner, repo1, "Hello World", "Hello World", "somebranch", pullRequestId, vcsutils.Closed)
	assert.NoError(t, err)

	err = client.UpdatePullRequest(ctx, owner, repo1, "Hello World", "Hello World", "somebranch", pullRequestId, "unknownState")
	assert.NoError(t, err)

	badClient, cleanUp := createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	err = badClient.UpdatePullRequest(ctx, "", repo1, "Hello World", "Hello World", "", pullRequestId, vcsutils.Open)
	assert.Error(t, err)
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
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "pullRequestComments", createAzureReposHandler)
	defer cleanUp()
	err = client.AddPullRequestComment(ctx, "", repo1, "test", 2)
	assert.NoError(t, err)

	badClient, cleanUp := createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	err = badClient.AddPullRequestComment(ctx, "", repo1, "test", 2)
	assert.Error(t, err)
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
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "getPullRequests", createAzureReposHandler)
	defer cleanUp()
	pullRequestsInfo, err := client.ListOpenPullRequests(ctx, "", repo1)
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(pullRequestsInfo, []PullRequestInfo{{ID: 1, Source: BranchInfo{Name: branch1, Repository: repo1}, Target: BranchInfo{Name: branch2, Repository: repo1}}}))

	badClient, cleanUp := createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	_, err = badClient.ListOpenPullRequests(ctx, "", repo1)
	assert.Error(t, err)

	// Test with body
	prBody := "hello world"
	res = ListOpenPullRequestsResponse{
		Value: []git.GitPullRequest{
			{
				PullRequestId: &pullRequestId,
				Description:   &prBody,
				Repository:    &git.GitRepository{Name: &repo1},
				SourceRefName: &branch1,
				TargetRefName: &branch2,
			},
		},
		Count: 1,
	}
	jsonRes, err = json.Marshal(res)
	assert.NoError(t, err)
	ctx = context.Background()
	client, cleanUp = createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "getPullRequests", createAzureReposHandler)
	defer cleanUp()
	pullRequestsInfo, err = client.ListOpenPullRequestsWithBody(ctx, "", repo1)
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(pullRequestsInfo, []PullRequestInfo{{ID: 1, Body: prBody, Source: BranchInfo{Name: branch1, Repository: repo1}, Target: BranchInfo{Name: branch2, Repository: repo1}}}))

	badClient, cleanUp = createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	_, err = badClient.ListOpenPullRequests(ctx, "", repo1)
	assert.Error(t, err)
}

func TestListPullRequestComments(t *testing.T) {
	type ListPullRequestCommentsResponse struct {
		Value []git.GitPullRequestCommentThread
		Count int
	}
	id1 := 1
	id2 := 2
	firstCommentContent := "first comment"
	secondCommentContent := "second comment"
	author := "test author"
	res := ListPullRequestCommentsResponse{
		Value: []git.GitPullRequestCommentThread{{
			Id:            &id1,
			PublishedDate: &azuredevops.Time{Time: time.Now()},
			Comments: &[]git.Comment{
				{
					Id:      &id1,
					Content: &firstCommentContent,
					Author:  &webapi.IdentityRef{DisplayName: &author},
				},
				{
					Id:      &id2,
					Content: &secondCommentContent,
					Author:  &webapi.IdentityRef{DisplayName: &author},
				},
			},
		}},
		Count: 1,
	}
	jsonRes, err := json.Marshal(res)
	assert.NoError(t, err)
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, jsonRes, "pullRequestComments", createAzureReposHandler)
	defer cleanUp()
	commentInfo, err := client.ListPullRequestComments(ctx, "", repo1, id1)
	expected := "Author: test author, Id: 1, Content:first comment\nAuthor: test author, Id: 2, Content:second comment\n"
	assert.Equal(t, expected, commentInfo[0].Content)
	assert.NoError(t, err)

	badClient, cleanUp := createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	_, err = badClient.ListPullRequestComments(ctx, "", repo1, id1)
	assert.Error(t, err)
}

func TestAzureRepos_TestGetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "azurerepos", "commits.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, response, "getCommits", createAzureReposHandler)
	defer cleanUp()

	commit, err := client.GetLatestCommit(ctx, "", repo1, branch1)
	assert.Equal(t, commit, CommitInfo{
		Hash:          "86d6919952702f9ab03bc95b45687f145a663de0",
		AuthorName:    "Test User",
		CommitterName: "Test User",
		Url:           "https://dev.azure.com/testuser/0b8072c4-ad86-4edb-a8f2-06dbc07e3e2d/_apis/git/repositories/94c1dba8-d9d9-4600-94b4-1a51acb43220/commits/86d6919952702f9ab03bc95b45687f145a663de0",
		Timestamp:     1667812601,
		Message:       "Updated package.json",
		AuthorEmail:   "testuser@jfrog.com",
	})
	assert.NoError(t, err)

	badClient, cleanUp := createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	_, err = badClient.GetLatestCommit(ctx, "", repo1, branch1)
	assert.Error(t, err)
}

func TestAzureRepos_TestGetCommits(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "azurerepos", "commits.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, response, "getCommits", createAzureReposHandler)
	defer cleanUp()

	commits, err := client.GetCommits(ctx, "", repo1, branch1)
	assert.Equal(t, CommitInfo{
		Hash:          "86d6919952702f9ab03bc95b45687f145a663de0",
		AuthorName:    "Test User",
		CommitterName: "Test User",
		Url:           "https://dev.azure.com/testuser/0b8072c4-ad86-4edb-a8f2-06dbc07e3e2d/_apis/git/repositories/94c1dba8-d9d9-4600-94b4-1a51acb43220/commits/86d6919952702f9ab03bc95b45687f145a663de0",
		Timestamp:     1667812601,
		Message:       "Updated package.json",
		AuthorEmail:   "testuser@jfrog.com",
	}, commits[0])
	assert.Equal(t, CommitInfo{
		Hash:          "4aa8367809020c4e97af29e2b57f7528d5d27702",
		AuthorName:    "Test User",
		CommitterName: "Test User",
		Url:           "https://dev.azure.com/testuser/0b8072c4-ad86-4edb-a8f2-06dbc07e3e2d/_apis/git/repositories/94c1dba8-d9d9-4600-94b4-1a51acb43220/commits/4aa8367809020c4e97af29e2b57f7528d5d27702",
		Timestamp:     1667201343,
		Message:       "Set up CI with Azure Pipelines",
		AuthorEmail:   "testuser@jfrog.com",
	}, commits[1])
	assert.Equal(t, CommitInfo{
		Hash:          "3779104c35804e15b6fdf4fee303e717cd6c1352",
		AuthorName:    "Test User",
		CommitterName: "Test User",
		Url:           "https://dev.azure.com/testuser/0b8072c4-ad86-4edb-a8f2-06dbc07e3e2d/_apis/git/repositories/94c1dba8-d9d9-4600-94b4-1a51acb43220/commits/3779104c35804e15b6fdf4fee303e717cd6c1352",
		Timestamp:     1667201200,
		Message:       "first commit",
		AuthorEmail:   "testuser@jfrog.com",
	}, commits[2])
	assert.NoError(t, err)

	badClient, cleanUp := createBadAzureReposClient(t, []byte{})
	defer cleanUp()
	_, err = badClient.GetCommits(ctx, "", repo1, branch1)
	assert.Error(t, err)
}

func TestAzureReposClient_AddSshKeyToRepository(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "getLatestCommit", createAzureReposHandler)
	defer cleanUp()
	assert.Error(t, client.AddSshKeyToRepository(ctx, owner, repo1, "", "", 0777))
}

func TestAzureReposClient_CreateLabel(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	assert.Error(t, client.CreateLabel(ctx, owner, repo1, LabelInfo{}))
}

func TestAzureReposClient_GetRepositoryInfo(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	_, err := client.GetRepositoryInfo(ctx, owner, repo1)
	assert.Error(t, err)
}

func TestAzureReposClient_GetCommitBySha(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	_, err := client.GetCommitBySha(ctx, owner, repo1, "")
	assert.Error(t, err)
}

func TestAzureReposClient_ListPullRequestLabels(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	_, err := client.ListPullRequestLabels(ctx, owner, repo1, 1)
	assert.Error(t, err)
}

func TestAzureReposClient_UnlabelPullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	err := client.UnlabelPullRequest(ctx, owner, repo1, "", 1)
	assert.Error(t, err)
}

func TestAzureReposClient_UploadCodeScanning(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	_, err := client.UploadCodeScanning(ctx, owner, repo1, "", "1")
	assert.Error(t, err)
}

func TestAzureReposClient_DownloadFileFromRepo(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "badTest", createAzureReposHandler)
	defer cleanUp()
	_, _, err := client.DownloadFileFromRepo(ctx, owner, repo1, "", "")
	assert.Error(t, err)
	client, cleanUp = createServerAndClient(t, vcsutils.AzureRepos, true, []byte("good"), "/_apis/ResourceAreas/DownloadFileFromRepo?includeContent=true&path=file.txt&versionDescriptor.version=&versionDescriptor.versionType=branch", createAzureReposHandler)
	defer cleanUp()
	content, statusCode, err := client.DownloadFileFromRepo(ctx, owner, repo1, "", "file.txt")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Equal(t, "good", string(content))
	client, cleanUp = createServerAndClient(t, vcsutils.AzureRepos, true, "", "bad^endpoint", createAzureReposHandler)
	defer cleanUp()
	_, statusCode, err = client.DownloadFileFromRepo(ctx, owner, repo1, "", "file.txt")
	assert.Error(t, err)
	assert.Equal(t, http.StatusNotFound, statusCode)
}

func TestAzureReposClient_CreateWebhook(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	_, _, err := client.CreateWebhook(ctx, owner, repo1, "", "1", vcsutils.PrRejected)
	assert.Error(t, err)
}

func TestAzureReposClient_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	err := client.UpdateWebhook(ctx, owner, repo1, "", "1", "", "", vcsutils.PrRejected)
	assert.Error(t, err)
}

func TestAzureReposClient_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	err := client.DeleteWebhook(ctx, owner, repo1, "")
	assert.Error(t, err)
}

func TestAzureReposClient_SetCommitStatus(t *testing.T) {
	ctx := context.Background()
	commitHash := "86d6919952702f9ab03bc95b45687f145a663de0"
	expectedUri := "/_apis/ResourceAreas/commitStatus"
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, nil, expectedUri, createAzureReposHandler)
	defer cleanUp()
	err := client.SetCommitStatus(ctx, 1, owner, repo1, commitHash, "", "", "")
	assert.NoError(t, err)

	badClient, badClientCleanup := createBadAzureReposClient(t, []byte{})
	defer badClientCleanup()
	err = badClient.SetCommitStatus(ctx, 1, owner, repo1, commitHash, "", "", "")
	assert.Error(t, err)
}

func TestAzureReposClient_GetLabel(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	_, err := client.GetLabel(ctx, owner, repo1, "")
	assert.Error(t, err)
}

func TestAzureReposClient_GetRepositoryEnvironmentInfo(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, "", "unsupportedTest", createAzureReposHandler)
	defer cleanUp()
	_, err := client.GetRepositoryEnvironmentInfo(ctx, owner, repo1, envName)
	assert.Error(t, err)
}

func TestGetUnsupportedInAzureError(t *testing.T) {
	functionName := "foo"
	assert.Error(t, getUnsupportedInAzureError(functionName))
	assert.Equal(t, "foo is currently not supported for Azure Repos", getUnsupportedInAzureError(functionName).Error())
}

func TestAzureReposClient_GetModifiedFiles(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "azurerepos", "compare_commits.json"))
		require.NoError(t, err)

		const expectedURI = "/_apis/ResourceAreas?%24skip=0&%24top=100&baseVersion=sha-1&diffCommonCommit=true&targetVersion=sha-2"
		client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, response, expectedURI, createAzureReposHandler)
		defer cleanUp()

		actual, err := client.GetModifiedFiles(ctx, "", repo1, "sha-1", "sha-2")
		require.NoError(t, err)
		assert.Equal(t, []string{
			"CustomerAddressModule/CustomerAddressModule.sln",
			"CustomerAddressModule/CustomerAddressModule/App.config",
			"CustomerAddressModule/CustomerAddressModule/CustomerAddressModule.csproj",
			"CustomerAddressModule/CustomerAddressModule/Form1.Designer.cs",
			"CustomerAddressModule/CustomerAddressModule/Form1.cs",
			"CustomerAddressModule/CustomerAddressModule/Form1.resx",
			"CustomerAddressModule/CustomerAddressModule/Program.cs",
			"CustomerAddressModule/CustomerAddressModule/Properties/AssemblyInfo.cs",
			"CustomerAddressModule/CustomerAddressModule/Properties/Resources.Designer.cs",
			"CustomerAddressModule/CustomerAddressModule/Properties/Resources.resx",
			"CustomerAddressModule/CustomerAddressModule/Properties/Settings.Designer.cs",
			"CustomerAddressModule/CustomerAddressModule/Properties/Settings.settings",
			"HelloWorld/.classpath",
			"HelloWorld/.project",
			"HelloWorld/build.xml",
			"HelloWorld/dist/lib/MyProject-20140210.jar",
			"HelloWorld/src/HelloWorld.java",
			"MyWebSite/MyWebSite/Views/Home/_Register.cshtml",
			"MyWebSite/MyWebSite/Web.config",
		}, actual)
	})

	t.Run("validation fails", func(t *testing.T) {
		client := AzureReposClient{}
		_, err := client.GetModifiedFiles(ctx, owner, "", "sha-1", "sha-2")
		require.Equal(t, errors.New("validation failed: required parameter 'repository' is missing"), err)
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "", "sha-2")
		require.Equal(t, errors.New("validation failed: required parameter 'refBefore' is missing"), err)
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "")
		require.Equal(t, errors.New("validation failed: required parameter 'refAfter' is missing"), err)
	})

	t.Run("failed request", func(t *testing.T) {
		const expectedURI = "/_apis/ResourceAreas?%24skip=0&%24top=100&baseVersion=sha-1&diffCommonCommit=true&targetVersion=sha-2"
		client, cleanUp := createServerAndClientReturningStatus(
			t,
			vcsutils.AzureRepos,
			true,
			nil,
			expectedURI,
			http.StatusInternalServerError,
			createAzureReposHandler,
		)
		defer cleanUp()

		_, err := client.GetModifiedFiles(ctx, "", repo1, "sha-1", "sha-2")
		require.EqualError(t, err, "null")
	})
}

func TestAzureReposClient_GetCommitStatus(t *testing.T) {
	ctx := context.Background()
	commitHash := "86d6919952702f9ab03bc95b45687f145a663de0"
	expectedUri := "/_apis/ResourceAreas/commitStatus"
	t.Run("Valid response", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "azurerepos", "commits_statuses.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, response, expectedUri, createAzureReposHandler)
		defer cleanUp()
		commitStatuses, err := client.GetCommitStatuses(ctx, owner, repo1, commitHash)
		assert.NoError(t, err)
		assert.True(t, len(commitStatuses) == 3)
		assert.True(t, commitStatuses[0].State == Pass)
		assert.True(t, commitStatuses[1].State == InProgress)
		assert.True(t, commitStatuses[2].State == Fail)
	})
	t.Run("Empty response", func(t *testing.T) {
		client, cleanUp := createServerAndClient(t, vcsutils.AzureRepos, true, nil, expectedUri, createAzureReposHandler)
		defer cleanUp()
		_, err := client.GetCommitStatuses(ctx, owner, repo1, commitHash)
		assert.NoError(t, err)
	})
	t.Run("Bad client", func(t *testing.T) {
		badClient, badClientCleanup := createBadAzureReposClient(t, []byte{})
		defer badClientCleanup()
		_, err := badClient.GetCommitStatuses(ctx, owner, repo1, "")
		assert.Error(t, err)
	})
}

func createAzureReposHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base64Token := base64.StdEncoding.EncodeToString([]byte(":" + token))
		assert.Equal(t, "Basic "+base64Token, r.Header.Get("Authorization"))
		if r.RequestURI == "/_apis" {
			jsonVal, err := os.ReadFile(filepath.Join("./", "testdata", "azurerepos", "resourcesResponse.json"))
			assert.NoError(t, err)
			_, err = w.Write(jsonVal)
			assert.NoError(t, err)
			return
		} else if r.RequestURI == "/_apis/ResourceAreas" {
			jsonVal := `{"value": [],"count": 0}`
			_, err := w.Write([]byte(jsonVal))
			assert.NoError(t, err)
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
	}
}

func createBadAzureReposClient(t *testing.T, response []byte) (VcsClient, func()) {
	client, cleanUp := createServerAndClient(
		t,
		vcsutils.AzureRepos,
		true,
		response,
		fmt.Sprintf("bad^endpoint/%s/_apis/git/repositories/%s/items/items?path=/&versionDescriptor[version]=%s&$format=zip",
			"",
			repo1,
			branch1),
		createAzureReposHandler)
	return client, cleanUp
}
