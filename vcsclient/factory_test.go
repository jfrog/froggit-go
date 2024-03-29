package vcsclient

import (
	"testing"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

const (
	apiEndpoint string = "apiEndpoint"
)

func TestClientBuilder(t *testing.T) {
	for _, vcsProvider := range []vcsutils.VcsProvider{vcsutils.GitHub, vcsutils.GitLab, vcsutils.BitbucketCloud, vcsutils.BitbucketServer, vcsutils.AzureRepos} {
		t.Run(vcsProvider.String(), func(t *testing.T) {
			clientBuilder := NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Username(username).Token(token).Project(project)
			assert.NotNil(t, clientBuilder)
			assert.Equal(t, vcsProvider, clientBuilder.vcsProvider)
			assert.Equal(t, apiEndpoint, clientBuilder.vcsInfo.APIEndpoint)
			assert.Equal(t, username, clientBuilder.vcsInfo.Username)
			assert.Equal(t, token, clientBuilder.vcsInfo.Token)
			assert.Equal(t, project, clientBuilder.vcsInfo.Project)
		})
	}
}

func TestNegativeGitlabClient(t *testing.T) {
	vcsClient, err := NewClientBuilder(vcsutils.GitLab).ApiEndpoint("https://bad^endpoint").Build()
	assert.Nil(t, vcsClient)
	assert.Error(t, err)
}

func TestNegativeBitbucketCloudClient(t *testing.T) {
	vcsClient, err := NewClientBuilder(vcsutils.BitbucketCloud).ApiEndpoint("https://bad^endpoint").Build()
	assert.Nil(t, vcsClient)
	assert.Error(t, err)
}
