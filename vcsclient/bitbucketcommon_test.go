package vcsclient

import (
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBitbucketClient_getBitbucketCommitState(t *testing.T) {
	assert.Equal(t, "SUCCESSFUL", getBitbucketCommitState(vcsutils.Pass))
	assert.Equal(t, "FAILED", getBitbucketCommitState(vcsutils.Fail))
	assert.Equal(t, "FAILED", getBitbucketCommitState(vcsutils.Error))
	assert.Equal(t, "INPROGRESS", getBitbucketCommitState(vcsutils.InProgress))
	assert.Equal(t, "", getBitbucketCommitState(5))
}

func TestBitbucketParseCommitStatuses(t *testing.T) {
	rawStatuses := map[string]interface{}{
		"values": []BitbucketCommitInfo{
			{
				State:       "SUCCESSFUL",
				Description: "Build successful",
				Url:         "http://example.com/build/1234",
				Title:       "jenkins",
				DateAdded:   1619189054828,
			},
			{
				State:       "FAILED",
				Description: "Build failed",
				Url:         "http://example.com/build/5678",
				Title:       "jenkins",
				DateAdded:   1619189055832,
			},
		},
	}

	provider := vcsutils.BitbucketServer
	expectedStatuses := []vcsutils.CommitStatusInfo{
		{
			State:       vcsutils.Pass,
			Description: "Build successful",
			DetailsUrl:  "http://example.com/build/1234",
			Creator:     "jenkins",
			CreatedAt:   time.Unix(1619189054, 828000000).UTC(),
		},
		{
			State:       vcsutils.Fail,
			Description: "Build failed",
			DetailsUrl:  "http://example.com/build/5678",
			Creator:     "jenkins",
			CreatedAt:   time.Unix(1619189055, 832000000).UTC(),
		},
	}

	statuses, err := bitbucketParseCommitStatuses(rawStatuses, provider)
	require.NoError(t, err)
	assert.Equal(t, expectedStatuses, statuses)
}

func TestGetCommitStatusInfoByBitbucketProvider_BitbucketServer(t *testing.T) {
	commitStatus := &BitbucketCommitInfo{
		State:       "SUCCESSFUL",
		Description: "Build successful",
		Url:         "http://example.com/build/1234",
		Title:       "jenkins",
		DateAdded:   1619189054828,
	}

	expectedStatus := vcsutils.CommitStatusInfo{
		State:       vcsutils.Pass,
		Description: "Build successful",
		DetailsUrl:  "http://example.com/build/1234",
		Creator:     "jenkins",
		CreatedAt:   time.Unix(1619189054, 828000000).UTC(),
	}

	status, err := getCommitStatusInfoByBitbucketProvider(commitStatus, vcsutils.BitbucketServer)
	require.NoError(t, err)
	assert.Equal(t, expectedStatus, status)
}

func TestGetCommitStatusInfoByBitbucketProvider_BitbucketCloud(t *testing.T) {
	commitStatus := &BitbucketCommitInfo{
		State:       "success",
		Description: "Test commit",
		Url:         "https://example.com/commit",
		Creator:     "John Doe",
		CreatedOn:   "2022-01-01T12:34:56.789Z",
		UpdatedOn:   "2022-01-02T23:45:01.234Z",
	}

	expectedResult := vcsutils.CommitStatusInfo{
		State:         vcsutils.Pass,
		Description:   "Test commit",
		DetailsUrl:    "https://example.com/commit",
		Creator:       "John Doe",
		CreatedAt:     time.Date(2022, 1, 1, 12, 34, 56, 789000000, time.UTC),
		LastUpdatedAt: time.Date(2022, 1, 2, 23, 45, 1, 234000000, time.UTC),
	}

	result, err := getCommitStatusInfoByBitbucketProvider(commitStatus, vcsutils.BitbucketCloud)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}
