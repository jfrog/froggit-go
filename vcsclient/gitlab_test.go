package vcsclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

func TestGitLabConnection(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []gitlab.Project{}, "/api/v4/projects", createGitLabHandler)
	defer cleanUp()

	err := client.TestConnection()
	assert.NoError(t, err)
}

func TestGitLabListRepositories(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []gitlab.Project{{Path: repo1}, {Path: repo2}}, "/api/v4/groups/frogger/projects?page=1", createGitLabHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories()
	assert.NoError(t, err)
	assert.Equal(t, actualRepositories, map[string][]string{username: {repo1, repo2}})
}

func TestGitLabListBranches(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, []gitlab.Branch{{Name: branch1}, {Name: branch2}}, fmt.Sprintf("/api/v4/projects/%s/repository/branches", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})
}

func TestGitLabCreateWebhook(t *testing.T) {
	id := rand.Int()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(owner, repo1, branch1, "https://jfrog.com", vcsutils.Push, vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, actualId, strconv.Itoa(id))
}

func TestGitLabUpdateWebhook(t *testing.T) {
	id := rand.Int()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks/%d", url.PathEscape(owner+"/"+repo1), id), createGitLabHandler)
	defer cleanUp()

	err := client.UpdateWebhook(owner, repo1, branch1, "https://jfrog.com", token, strconv.Itoa(id), vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestGitLabDeleteWebhook(t *testing.T) {
	id := rand.Int()
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.ProjectHook{ID: id}, fmt.Sprintf("/api/v4/projects/%s/hooks/%d", url.PathEscape(owner+"/"+repo1), id), createGitLabHandler)
	defer cleanUp()

	err := client.DeleteWebhook(owner, repo1, strconv.Itoa(id))
	assert.NoError(t, err)
}

func TestGitLabCreateCommitStatus(t *testing.T) {
	ref := "5fbf81b31ff7a3b06bd362d1891e2f01bdb2be69"
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, gitlab.CommitStatus{}, fmt.Sprintf("/api/v4/projects/%s/statuses/%s", url.PathEscape(owner+"/"+repo1), ref), createGitLabHandler)
	defer cleanUp()

	err := client.SetCommitStatus(InProgress, owner, repo1, ref, "Commit status title", "Commit status description", "https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestGitLabDownloadRepository(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	repoFile, err := os.ReadFile(filepath.Join("testdata", "gitlab", "hello-world-main.tar.gz"))
	assert.NoError(t, err)

	ref := "5fbf81b31ff7a3b06bd362d1891e2f01bdb2be69"
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, repoFile, fmt.Sprintf("/api/v4/projects/%s/repository/archive.tar.gz?sha=%s", url.PathEscape(owner+"/"+repo1), ref), createGitLabHandler)
	defer cleanUp()

	err = client.DownloadRepository(owner, repo1, ref, dir)
	assert.NoError(t, err)
	fileinfo, err := ioutil.ReadDir(dir)
	assert.Len(t, fileinfo, 1)
	assert.Equal(t, "README.md", fileinfo[0].Name())
}

func TestGitLabCreatePullRequest(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.GitLab, false, &gitlab.MergeRequest{}, fmt.Sprintf("/api/v4/projects/%s/merge_requests", url.PathEscape(owner+"/"+repo1)), createGitLabHandler)
	defer cleanUp()

	err := client.CreatePullRequest(owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func createGitLabHandler(t *testing.T, expectedUri string, response []byte) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.RequestURI == "/api/v4/" {
			return
		}
		if r.RequestURI == "/api/v4/groups" {
			byteResponse, err := json.Marshal(&[]gitlab.Group{{Path: username}})
			assert.NoError(t, err)
			_, err = w.Write(byteResponse)
			assert.NoError(t, err)
			return
		}
		_, err := w.Write(response)
		assert.NoError(t, err)
		assert.Equal(t, expectedUri, r.RequestURI)
		assert.Equal(t, token, r.Header.Get("Private-Token"))
	})
}
