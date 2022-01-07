package vcsutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVcsProviderString(t *testing.T) {
	assert.Equal(t, "GitHub", GitHub.String())
	assert.Equal(t, "GitLab", GitLab.String())
	assert.Equal(t, "Bitbucket Server", BitbucketServer.String())
	assert.Equal(t, "Bitbucket Cloud", BitbucketCloud.String())
	assert.Equal(t, "", (VcsProvider(5)).String())
}
