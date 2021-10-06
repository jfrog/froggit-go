package vcsclient

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-github/v38/github"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

func TestGitHubConnection(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, "It's Not Easy Being Green", "/zen", createGitHubHandler)
	defer cleanUp()

	err := client.TestConnection()
	assert.NoError(t, err)
}

func TestGitHubListRepositories(t *testing.T) {
	expectedRepo1 := github.Repository{Name: &repo1, Owner: &github.User{Login: &username}}
	expectedRepo2 := github.Repository{Name: &repo2, Owner: &github.User{Login: &username}}
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []github.Repository{expectedRepo1, expectedRepo2}, "/user/repos", createGitHubHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories()
	assert.NoError(t, err)
	assert.Equal(t, actualRepositories, map[string][]string{username: {repo1, repo2}})
}

func TestGitHubListBranches(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []github.Branch{{Name: &branch1}, {Name: &branch2}}, fmt.Sprintf("/repos/jfrog/%s/branches", repo1), createGitHubHandler)
	defer cleanUp()

	actualBranches, err := client.ListBranches(owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualBranches, []string{branch1, branch2})
}

func TestGitHubCreateWebhook(t *testing.T) {
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks", repo1), createGitHubHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(owner, repo1, branch1, "https://jfrog.com", vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, actualId, strconv.FormatInt(id, 10))
}

func TestGitHubUpdateWebhook(t *testing.T) {
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks/%s", repo1, strconv.FormatInt(id, 10)), createGitHubHandler)
	defer cleanUp()

	err := client.UpdateWebhook(owner, repo1, branch1, "https://jfrog.com", token, strconv.FormatInt(id, 10), vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestGitHubDeleteWebhook(t *testing.T) {
	id := rand.Int63()
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.Hook{ID: &id}, fmt.Sprintf("/repos/jfrog/%s/hooks/%s", repo1, strconv.FormatInt(id, 10)), createGitHubHandler)
	defer cleanUp()

	err := client.DeleteWebhook(owner, repo1, strconv.FormatInt(id, 10))
	assert.NoError(t, err)
}

func TestGitHubCreateCommitStatus(t *testing.T) {
	ref := "39e5418"
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.RepoStatus{}, fmt.Sprintf("/repos/jfrog/%s/statuses/%s", repo1, ref), createGitHubHandler)
	defer cleanUp()

	err := client.SetCommitStatus(Error, owner, repo1, ref, "Commit status title", "Commit status description", "https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestGitHubDownloadRepository(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, []byte("https://github.com/octocat/Hello-World/archive/refs/heads/master.tar.gz"), "/repos/jfrog/Hello-World/tarball/test", createGitHubHandler)
	defer cleanUp()
	assert.NoError(t, err)

	err = client.DownloadRepository(owner, "Hello-World", "test", dir)
	assert.NoError(t, err)
	fileinfo, err := ioutil.ReadDir(dir)
	assert.Len(t, fileinfo, 1)
	assert.Equal(t, "README", fileinfo[0].Name())
}

func TestGitHubCreatePullRequest(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, github.PullRequest{}, "/repos/jfrog/repo-1/pulls", createGitHubHandler)
	defer cleanUp()

	err := client.CreatePullRequest(owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func createGitHubHandler(t *testing.T, expectedUri string, response []byte) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "tarball") {
			w.Header().Add("Location", string(response))
			w.WriteHeader(http.StatusFound)
			return
		} else {
			w.WriteHeader(http.StatusOK)
		}
		w.Write(response)
		assert.Equal(t, expectedUri, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	})
}
