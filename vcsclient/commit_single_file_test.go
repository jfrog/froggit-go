package vcsclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v74/github"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

func TestGitHubClient_CommitAndPushFiles_SingleFileCreate(t *testing.T) {
	ctx := context.Background()
	path := ".github/workflows/jfrog-github-oidc-example.yml"
	var putHadSHA bool

	handler := func(t *testing.T, _ string, _ []byte, _ int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
			switch {
			case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/"):
				w.WriteHeader(http.StatusNotFound)
				_, err := w.Write([]byte(`{"message":"Not Found","documentation_url":"https://docs.github.com/rest"}`))
				assert.NoError(t, err)
			case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				var opts github.RepositoryContentFileOptions
				assert.NoError(t, json.Unmarshal(body, &opts))
				putHadSHA = opts.SHA != nil && *opts.SHA != ""
				if putHadSHA {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, err = w.Write([]byte(`{"message":"sha unexpectedly supplied for create"}`))
					assert.NoError(t, err)
					return
				}
				w.WriteHeader(http.StatusCreated)
				_, err = w.Write(mustMarshal(&github.RepositoryContentResponse{}))
				assert.NoError(t, err)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.RequestURI)
			}
		}
	}

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, "", handler)
	defer cleanUp()

	err := client.CommitAndPushFiles(ctx, owner, repo1, "feature/oidc-example",
		"Add OIDC example workflow", "example", "example@jfrog.com",
		[]FileToCommit{{Path: path, Content: "name: jfrog-github-oidc-example\n"}})
	assert.NoError(t, err)
	assert.False(t, putHadSHA)
}

func TestGitHubClient_CommitAndPushFiles_SingleFileUpdate(t *testing.T) {
	ctx := context.Background()
	path := ".github/workflows/jfrog-github-oidc-example.yml"
	existingSHA := "existingblobsha1234567890abcdef1234567890ab"
	var putSHA string

	handler := func(t *testing.T, _ string, _ []byte, _ int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
			switch {
			case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/"):
				w.WriteHeader(http.StatusOK)
				_, err := w.Write(mustMarshal(&github.RepositoryContent{
					SHA:  github.Ptr(existingSHA),
					Path: github.Ptr(path),
					Name: github.Ptr("jfrog-github-oidc-example.yml"),
					Type: github.Ptr("file"),
				}))
				assert.NoError(t, err)
			case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				var opts github.RepositoryContentFileOptions
				assert.NoError(t, json.Unmarshal(body, &opts))
				if opts.SHA == nil || *opts.SHA == "" {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, err = w.Write([]byte(`{"message":"Invalid request.\n\n\"sha\" wasn't supplied.","documentation_url":"https://docs.github.com/rest/repos/contents"}`))
					assert.NoError(t, err)
					return
				}
				putSHA = *opts.SHA
				w.WriteHeader(http.StatusOK)
				_, err = w.Write(mustMarshal(&github.RepositoryContentResponse{}))
				assert.NoError(t, err)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.RequestURI)
			}
		}
	}

	client, cleanUp := createServerAndClient(t, vcsutils.GitHub, false, nil, "", handler)
	defer cleanUp()

	err := client.CommitAndPushFiles(ctx, owner, repo1, "feature/oidc-example",
		"Update OIDC example workflow", "example", "example@jfrog.com",
		[]FileToCommit{{Path: path, Content: "name: jfrog-github-oidc-example\non:\n  workflow_dispatch:\n"}})
	assert.NoError(t, err)
	assert.Equal(t, existingSHA, putSHA)
}
