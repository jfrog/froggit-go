package vcsclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

func TestBitbucketServerConnection(t *testing.T) {
	mockRespose := make(map[string][]bitbucketv1.User)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, mockRespose, "/api/1.0/admin/users", createBitbucketServerHandler)
	defer cleanUp()

	err := client.TestConnection()
	assert.NoError(t, err)
}

func TestBitbucketServerListRepositories(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, "", createBitbucketServerListRepositoriesHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories()
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"~" + username: {repo1}, username: {repo2}}, actualRepositories)
}

func TestBitbucketServerListBranches(t *testing.T) {
	mockRespose := map[string][]bitbucketv1.Branch{
		"values": {{ID: branch1}, {ID: branch2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, mockRespose, "/api/1.0/projects/jfrog/repos/repo-1/branches", createBitbucketServerHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})
}

func TestBitbucketServerCreateWebhook(t *testing.T) {
	id := rand.Int31()
	mockRespose := bitbucketv1.Webhook{ID: int(id)}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, mockRespose, "/api/1.0/projects/jfrog/repos/repo-1/webhooks", createBitbucketServerHandler)
	defer cleanUp()

	actualId, token, err := client.CreateWebhook(owner, repo1, branch1, "https://httpbin.org/anything", vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, strconv.Itoa(int(id)), actualId)
}

func TestBitbucketServerUpdateWebhook(t *testing.T) {
	id := rand.Int31()
	stringId := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringId), createBitbucketServerHandler)
	defer cleanUp()

	err := client.UpdateWebhook(owner, repo1, branch1, "https://httpbin.org/anything", token, stringId, vcsutils.PrCreated, vcsutils.PrEdited)
	assert.NoError(t, err)
}

func TestBitbucketServerDeleteWebhook(t *testing.T) {
	id := rand.Int31()
	stringId := strconv.Itoa(int(id))

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/api/1.0/projects/jfrog/repos/repo-1/webhooks/%s", stringId), createBitbucketServerHandler)
	defer cleanUp()

	err := client.DeleteWebhook(owner, repo1, stringId)
	assert.NoError(t, err)
}

func TestBitbucketServerSetCommitStatus(t *testing.T) {
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, false, nil, fmt.Sprintf("/build-status/1.0/commits/%s", ref), createBitbucketServerHandler)
	defer cleanUp()

	err := client.SetCommitStatus(Fail, owner, repo1, ref, "Commit status title", "Commit status description", "https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestBitbucketServerDownloadRepository(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	client, err := NewClientBuilder(vcsutils.BitbucketServer).ApiEndpoint("https://open-bitbucket.nrao.edu/rest").Build()
	assert.NoError(t, err)

	err = client.DownloadRepository("ssa", "solr-system", "master", dir)
	assert.NoError(t, err)

	_, err = os.OpenFile(filepath.Join(dir, "README.md"), os.O_RDONLY, 0644)
	assert.NoError(t, err)
}

func TestBitbucketServerCreatePullRequest(t *testing.T) {
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketServer, true, nil, "/api/1.0/projects/jfrog/repos/repo-1/pull-requests", createBitbucketServerHandler)
	defer cleanUp()

	err := client.CreatePullRequest(owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func createBitbucketServerHandler(t *testing.T, expectedUri string, response []byte) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(response)
		assert.Equal(t, expectedUri, r.RequestURI)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	})
}

func createBitbucketServerListRepositoriesHandler(t *testing.T, expectedUri string, response []byte) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var responseObj interface{}
		if r.RequestURI == "/api/1.0/projects?start=0" {
			responseObj = map[string][]bitbucketv1.Project{"values": {{Key: username}}}
			w.Header().Add("X-Ausername", username)

		} else if r.RequestURI == "/api/1.0/projects/~frogger/repos?start=0" {
			responseObj = map[string][]bitbucketv1.Repository{"values": {{Slug: repo1}}}
		} else if r.RequestURI == "/api/1.0/projects/frogger/repos?start=0" {
			responseObj = map[string][]bitbucketv1.Repository{"values": {{Slug: repo2}}}
		} else {
			assert.Fail(t, "Unexpected request Uri "+r.RequestURI)
		}
		w.WriteHeader(http.StatusOK)
		response, err := json.Marshal(responseObj)
		assert.NoError(t, err)
		w.Write(response)
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
	})
}
