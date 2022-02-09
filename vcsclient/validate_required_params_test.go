package vcsclient

import (
	"context"
	"fmt"
	"testing"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequiredParams_AddSshKeyInvalidPayload(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		keyName       string
		publicKey     string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository", "key name", "public key"}},
		{name: "empty owner", repo: "repo", keyName: "my key", publicKey: "ssh-rsa AAAA..", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", keyName: "my key", publicKey: "ssh-rsa AAAA..", missingParams: []string{"repository"}},
		{name: "empty keyName", owner: "owner", repo: "repo", publicKey: "ssh-rsa AAAA..", missingParams: []string{"key name"}},
		{name: "empty publicKey", owner: "owner", repo: "repo", keyName: "my key", missingParams: []string{"public key"}},
	}

	for _, p := range getAllProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				err := client.AddSshKeyToRepository(ctx, tt.owner, tt.repo, tt.keyName, tt.publicKey, Read)
				assertMissingParam(t, err, tt.missingParams...)
			})
		}
	}
}

func TestRequiredParams_GetLatestCommitInvalidPayload(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		branch        string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository", "branch"}},
		{name: "empty owner", repo: "repo", branch: "branch", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", branch: "branch", missingParams: []string{"repository"}},
		{name: "empty branch", owner: "owner", repo: "repo", missingParams: []string{"branch"}},
	}

	for _, p := range getAllProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				result, err := client.GetLatestCommit(ctx, tt.owner, tt.repo, tt.branch)
				assertMissingParam(t, err, tt.missingParams...)
				assert.Empty(t, result)
			})
		}
	}
}

func TestRequiredParams_GetRepositoryInfoInvalidPayload(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository"}},
		{name: "empty owner", repo: "repo", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", missingParams: []string{"repository"}},
	}

	for _, p := range getAllProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				result, err := client.GetRepositoryInfo(ctx, tt.owner, tt.repo)
				assertMissingParam(t, err, tt.missingParams...)
				assert.Empty(t, result)
			})
		}
	}
}

func TestRequiredParams_GetCommitByShaInvalidPayload(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		sha           string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository", "sha"}},
		{name: "empty owner", repo: "repo", sha: "sha", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", sha: "sha", missingParams: []string{"repository"}},
		{name: "empty branch", owner: "owner", repo: "repo", missingParams: []string{"sha"}},
	}

	for _, p := range getAllProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				result, err := client.GetCommitBySha(ctx, tt.owner, tt.repo, tt.sha)
				assertMissingParam(t, err, tt.missingParams...)
				assert.Empty(t, result)
			})
		}
	}
}

func TestRequiredParams_AddPullRequestComment(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		content       string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository", "content"}},
		{name: "empty owner", repo: "repo", content: "content", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", content: "content", missingParams: []string{"repository"}},
		{name: "empty content", owner: "owner", missingParams: []string{"content"}},
	}

	for _, p := range getAllProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				err := client.AddPullRequestComment(ctx, tt.owner, tt.repo, tt.content, 0)
				assertMissingParam(t, err, tt.missingParams...)
			})
		}
	}
}

func TestRequiredParams_CreateLabel(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		labelInfo     LabelInfo
		missingParams []string
	}{
		{name: "all empty", labelInfo: LabelInfo{}, missingParams: []string{"owner", "repository", "LabelInfo.name"}},
		{name: "empty owner", repo: "repo", labelInfo: LabelInfo{Name: "name"}, missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", labelInfo: LabelInfo{Name: "name"}, missingParams: []string{"repository"}},
		{name: "empty LabelInfo.name", owner: "owner", repo: "repo", labelInfo: LabelInfo{}, missingParams: []string{"LabelInfo.name"}},
	}

	for _, p := range getNonBitbucketProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				err := client.CreateLabel(ctx, tt.owner, tt.repo, tt.labelInfo)
				assertMissingParam(t, err, tt.missingParams...)
			})
		}
	}
}

func TestRequiredParams_GetLabel(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		labelName     string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository", "name"}},
		{name: "empty owner", repo: "repo", labelName: "name", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", labelName: "name", missingParams: []string{"repository"}},
		{name: "empty name", owner: "owner", repo: "repo", missingParams: []string{"name"}},
	}

	for _, p := range getNonBitbucketProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				result, err := client.GetLabel(ctx, tt.owner, tt.repo, tt.labelName)
				assertMissingParam(t, err, tt.missingParams...)
				assert.Empty(t, result)
			})
		}
	}
}

func TestRequiredParams_ListPullRequestLabels(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository"}},
		{name: "empty owner", repo: "repo", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", missingParams: []string{"repository"}},
	}

	for _, p := range getNonBitbucketProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				result, err := client.ListPullRequestLabels(ctx, tt.owner, tt.repo, 0)
				assertMissingParam(t, err, tt.missingParams...)
				assert.Empty(t, result)
			})
		}
	}
}

func TestRequiredParams_UnlabelPullRequest(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		labelName     string
		missingParams []string
	}{
		{name: "all empty", missingParams: []string{"owner", "repository"}},
		{name: "empty owner", repo: "repo", missingParams: []string{"owner"}},
		{name: "empty repo", owner: "owner", missingParams: []string{"repository"}},
	}

	for _, p := range getNonBitbucketProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx, client := createClientAndContext(t, p)
				err := client.UnlabelPullRequest(ctx, tt.owner, tt.repo, tt.labelName, 0)
				assertMissingParam(t, err, tt.missingParams...)
			})
		}
	}
}

func createClientAndContext(t *testing.T, provider vcsutils.VcsProvider) (context.Context, VcsClient) {
	ctx := context.Background()
	client, err := NewClientBuilder(provider).Build()
	require.NoError(t, err)
	return ctx, client
}

func assertMissingParam(t *testing.T, err error, missingParam ...string) {
	assert.Error(t, err)
	message := err.Error()
	for _, param := range missingParam {
		assert.Contains(t, message, fmt.Sprintf("required parameter '%s' is missing", param))
	}
}
