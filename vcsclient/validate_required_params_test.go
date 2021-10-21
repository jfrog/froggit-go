package vcsclient

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRequiredParams_AddSshKeyInvalidPayload(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		keyName    string
		publicKey  string
		assertFunc func(*testing.T, error)
	}{
		{
			name:      "all empty",
			owner:     "",
			repo:      "",
			keyName:   "",
			publicKey: "",
			assertFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				message := err.Error()
				assert.Contains(t, message, "required parameter 'owner' is missing")
				assert.Contains(t, message, "required parameter 'repository' is missing")
				assert.Contains(t, message, "required parameter 'key name' is missing")
				assert.Contains(t, message, "required parameter 'public key' is missing")
			},
		},
		{
			name:      "empty owner",
			owner:     "",
			repo:      "repo",
			keyName:   "my key",
			publicKey: "ssh-rsa AAAA..",
			assertFunc: func(t *testing.T, err error) {
				assert.EqualError(t, err, "validation failed: required parameter 'owner' is missing")
			},
		},
		{
			name:      "empty repo",
			owner:     "owner",
			repo:      "",
			keyName:   "my key",
			publicKey: "ssh-rsa AAAA..",
			assertFunc: func(t *testing.T, err error) {
				assert.EqualError(t, err, "validation failed: required parameter 'repository' is missing")
			},
		},
		{
			name:      "empty keyName",
			owner:     "owner",
			repo:      "repo",
			keyName:   "",
			publicKey: "ssh-rsa AAAA..",
			assertFunc: func(t *testing.T, err error) {
				assert.EqualError(t, err, "validation failed: required parameter 'key name' is missing")
			},
		},
		{
			name:      "empty publicKey",
			owner:     "owner",
			repo:      "repo",
			keyName:   "my key",
			publicKey: "",
			assertFunc: func(t *testing.T, err error) {
				assert.EqualError(t, err, "validation failed: required parameter 'public key' is missing")
			},
		},
	}

	for _, p := range getAllProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx := context.Background()
				client, err := NewClientBuilder(p).Build()
				require.NoError(t, err)
				err = client.AddSshKeyToRepository(ctx, tt.owner, tt.repo, tt.keyName, tt.publicKey, Read)
				tt.assertFunc(t, err)
			})
		}
	}
}

func TestRequiredParams__GetLatestCommitInvalidPayload(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		branch     string
		assertFunc func(*testing.T, error)
	}{
		{
			name:   "all empty",
			owner:  "",
			repo:   "",
			branch: "",
			assertFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				message := err.Error()
				assert.Contains(t, message, "required parameter 'owner' is missing")
				assert.Contains(t, message, "required parameter 'repository' is missing")
				assert.Contains(t, message, "required parameter 'branch' is missing")
			},
		},
		{
			name:   "empty owner",
			owner:  "",
			repo:   "repo",
			branch: "branch",
			assertFunc: func(t *testing.T, err error) {
				assert.EqualError(t, err, "validation failed: required parameter 'owner' is missing")
			},
		},
		{
			name:   "empty repo",
			owner:  "owner",
			repo:   "",
			branch: "branch",
			assertFunc: func(t *testing.T, err error) {
				assert.EqualError(t, err, "validation failed: required parameter 'repository' is missing")
			},
		},
		{
			name:   "empty branch",
			owner:  "owner",
			repo:   "repo",
			branch: "",
			assertFunc: func(t *testing.T, err error) {
				assert.EqualError(t, err, "validation failed: required parameter 'branch' is missing")
			},
		},
	}

	for _, p := range getAllProviders() {
		for _, tt := range tests {
			t.Run(p.String()+" "+tt.name, func(t *testing.T) {
				ctx := context.Background()
				client, err := NewClientBuilder(p).Build()
				require.NoError(t, err)

				result, err := client.GetLatestCommit(ctx, tt.owner, tt.repo, tt.branch)
				tt.assertFunc(t, err)
				assert.Empty(t, result)
			})
		}
	}
}
