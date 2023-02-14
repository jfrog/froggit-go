package webhookparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBranchStatus(t *testing.T) {
	assert.Equal(t, WebhookInfoBranchStatusDeleted, branchStatus(true, false))
	assert.Equal(t, WebhookInfoBranchStatusCreated, branchStatus(false, true))
	assert.Equal(t, WebhookInfoBranchStatusUpdated, branchStatus(true, true))
	// this one should never happen
	assert.Equal(t, WebhookInfoBranchStatusUpdated, branchStatus(false, false))
}
