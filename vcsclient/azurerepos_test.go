package vcsclient

import (
	"context"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestAzureReposConnection(t *testing.T) {
	ctx := context.Background()
	vcsInfo := VcsInfo{
		APIEndpoint: "https://dev.azure.com/",
		Username:    "omerzidkoni",
		Token:       "62ylhphw2lmyjsacpjirbsfdbimov5kulcbpa5eqpwtaf6gnpuoa",
	}
	client, err := NewAzureReposClient(vcsInfo)
	assert.NoError(t, err)
	err = client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestListRepositories(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.AzureRepos).Username("omerzidkoni").Token("62ylhphw2lmyjsacpjirbsfdbimov5kulcbpa5eqpwtaf6gnpuoa").ApiEndpoint("https://dev.azure.com/").Project("test_frogbot").Build()
	assert.NoError(t, err)
	_, err = client.ListRepositories(ctx)
	assert.NoError(t, err)
}

func TestListBranches(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.AzureRepos).Username("omerzidkoni").Token("62ylhphw2lmyjsacpjirbsfdbimov5kulcbpa5eqpwtaf6gnpuoa").ApiEndpoint("https://dev.azure.com/").Project("test_frogbot").Build()
	assert.NoError(t, err)
	_, err = client.ListBranches(ctx, "", "test_frogbot")
	assert.NoError(t, err)
}

func TestDownloadRepository(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.AzureRepos).Username("omerzidkoni").Token("62ylhphw2lmyjsacpjirbsfdbimov5kulcbpa5eqpwtaf6gnpuoa").ApiEndpoint("https://dev.azure.com/").Project("test_frogbot").Build()
	assert.NoError(t, err)
	wd, err := os.Getwd()

	err = client.DownloadRepository(ctx, "", "test_frogbot", "master", wd)
}
