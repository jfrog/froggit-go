package vcsclient

import (
	"log"
	"testing"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

const (
	apiEndpoint string = "apiEndpoint"
)

func TestClientBuilder(t *testing.T) {
	for _, vcsProvider := range []vcsutils.VcsProvider{vcsutils.GitHub, vcsutils.GitLab, vcsutils.BitbucketCloud, vcsutils.BitbucketServer} {
		t.Run(vcsProvider.String(), func(t *testing.T) {
			clientBuilder := NewClientBuilder(vcsProvider).Logger(log.Default()).ApiEndpoint(apiEndpoint).Username(username).Token(token)
			assert.NotNil(t, clientBuilder)
			assert.Equal(t, vcsProvider, clientBuilder.vcsProvider)
			assert.Equal(t, log.Default(), clientBuilder.logger)
			assert.Equal(t, apiEndpoint, clientBuilder.vcsInfo.ApiEndpoint)
			assert.Equal(t, username, clientBuilder.vcsInfo.Username)
			assert.Equal(t, token, clientBuilder.vcsInfo.Token)
		})
	}
}

func TestNegativeGitlabClient(t *testing.T) {
	vcsClient, err := NewClientBuilder(vcsutils.GitLab).Logger(&log.Logger{}).ApiEndpoint("https://bad^endpoint").Build()
	assert.Nil(t, vcsClient)
	assert.Error(t, err)
}

func TestNegativeBitbucketCloudClient(t *testing.T) {
	vcsClient, err := NewClientBuilder(vcsutils.BitbucketCloud).Logger(&log.Logger{}).ApiEndpoint("https://bad^endpoint").Build()
	assert.Nil(t, vcsClient)
	assert.Error(t, err)
}
