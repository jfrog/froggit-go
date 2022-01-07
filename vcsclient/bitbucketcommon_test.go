package vcsclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitbucketClient_getBitbucketCommitState(t *testing.T) {
	assert.Equal(t, "SUCCESSFUL", getBitbucketCommitState(Pass))
	assert.Equal(t, "FAILED", getBitbucketCommitState(Fail))
	assert.Equal(t, "FAILED", getBitbucketCommitState(Error))
	assert.Equal(t, "INPROGRESS", getBitbucketCommitState(InProgress))
	assert.Equal(t, "", getBitbucketCommitState(5))
}
