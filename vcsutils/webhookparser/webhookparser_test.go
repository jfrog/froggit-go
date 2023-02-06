package webhookparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBranchStatus(t *testing.T) {
	assert.Equal(t, WebhookinfobranchstatusDeleted, branchStatus(true, false))
	assert.Equal(t, WebhookinfobranchstatusCreated, branchStatus(false, true))
	assert.Equal(t, WebhookinfobranchstatusUpdated, branchStatus(true, true))
	// this one should never happen
	assert.Equal(t, WebhookinfobranchstatusUpdated, branchStatus(false, false))
}
