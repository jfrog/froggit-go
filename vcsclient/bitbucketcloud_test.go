package vcsclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/ktrysmt/go-bitbucket"
	"github.com/stretchr/testify/assert"
)

func TestBitbucketCloudConnection(t *testing.T) {
	mockRespose := map[string][]bitbucket.User{"values": {}}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockRespose, "/user", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.TestConnection()
	assert.NoError(t, err)
}

func TestBitbucketCloudListRepositories(t *testing.T) {
	mockRespose := map[string][]bitbucket.Repository{
		"values": {{Slug: repo1}, {Slug: repo2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockRespose, "/repositories/"+username, createBitbucketCloudHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories()
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{username: {repo1, repo2}}, actualRepositories)
}

func TestBitbucketCloudListBranches(t *testing.T) {
	mockRespose := map[string][]bitbucket.BranchModel{
		"values": {{Name: branch1}, {Name: branch2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockRespose, "/repositories/jfrog/repo-1/refs/branches?", createBitbucketCloudHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})
}

func TestBitbucketCloudCreateWebhook(t *testing.T) {
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	mockRespose := bitbucket.WebhooksOptions{Uuid: "{" + id.String() + "}"}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockRespose, "/repositories/jfrog/repo-1/hooks", createBitbucketCloudHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(owner, repo1, branch1, "https://httpbin.org/anything", vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, id.String(), actualId)
}

func TestBitbucketCloudUpdateWebhook(t *testing.T) {
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)
	defer cleanUp()

	err = client.UpdateWebhook(owner, repo1, branch1, "https://httpbin.org/anything", token, id.String(), vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestBitbucketCloudDeleteWebhook(t *testing.T) {
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)

	defer cleanUp()

	err = client.DeleteWebhook(owner, repo1, id.String())
	assert.NoError(t, err)
}

func TestBitbucketCloudSetCommitStatus(t *testing.T) {
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/commit/%s/statuses/build", ref), createBitbucketCloudHandler)
	defer cleanUp()

	err := client.SetCommitStatus(Pass, owner, repo1, ref, "Commit status title", "Commit status description", "https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestBitbucketCloudDownloadRepository(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.DownloadRepository(owner, "jfrog-setup-cli", "master", dir)
	assert.NoError(t, err)
	rootFiles, err := ioutil.ReadDir(dir)
	assert.NotEmpty(t, rootFiles)
	readmeFound := false
	for _, file := range rootFiles {
		if file.Name() == "README.md" {
			readmeFound = true
		}
	}
	assert.True(t, readmeFound)
}

func TestBitbucketCloudCreatePullRequest(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, "/repositories/jfrog/repo-1/pullrequests/", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.CreatePullRequest(owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func createBitbucketCloudHandler(t *testing.T, expectedUri string, response []byte) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.RequestURI == "/workspaces" {
			workspacesResults := make(map[string]interface{})
			workspacesResults["values"] = []bitbucket.Workspace{{Slug: username}}
			response, err := json.Marshal(workspacesResults)
			assert.NoError(t, err)
			w.Write(response)
		} else {
			w.Write(response)
			assert.Equal(t, expectedUri, r.RequestURI)
		}
		assert.Equal(t, basicAuthHeader, r.Header.Get("Authorization"))
	})
}
