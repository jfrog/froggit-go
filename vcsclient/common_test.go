package vcsclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/stretchr/testify/assert"
)

const (
	owner           = "jfrog"
	token           = "abc123"
	basicAuthHeader = "Basic ZnJvZ2dlcjphYmMxMjM="
)

var (
	repo1    = "repo-1"
	repo2    = "repo-2"
	username = "frogger"
	branch1  = "branch-1"
	branch2  = "branch-2"
)

type createHandlerFunc func(t *testing.T, expectedUri string, response []byte, expectedStatusCode int) http.HandlerFunc
type createPostHandlerFunc func(t *testing.T, expectedUri string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHttpMethod string) http.HandlerFunc

func createServerAndClient(t *testing.T, vcsProvider vcsutils.VcsProvider, basicAuth bool, response interface{},
	expectedURI string, createHandlerFunc createHandlerFunc) (VcsClient, func()) {
	return createServerAndClientReturningStatus(t, vcsProvider, basicAuth, response, expectedURI, http.StatusOK, createHandlerFunc)
}

func createServerAndClientReturningStatus(t *testing.T, vcsProvider vcsutils.VcsProvider, basicAuth bool, response interface{},
	expectedURI string, expectedStatusCode int, createHandlerFunc createHandlerFunc) (VcsClient, func()) {
	var byteResponse []byte
	var ok bool
	if byteResponse, ok = response.([]byte); !ok {
		// Response is not a byte array - unmarshal is needed
		var err error
		byteResponse, err = json.Marshal(response)
		assert.NoError(t, err)
	}
	server := httptest.NewServer(createHandlerFunc(t, expectedURI, byteResponse, expectedStatusCode))
	client := buildClient(t, vcsProvider, basicAuth, server)
	return client, server.Close
}

func createBodyHandlingServerAndClient(t *testing.T, vcsProvider vcsutils.VcsProvider, basicAuth bool, response interface{},
	expectedURI string, expectedStatusCode int, expectedRequestBody []byte, expectedHTTPMethod string,
	createPostHandlerFunc createPostHandlerFunc) (VcsClient, func()) {
	var byteResponse []byte
	var ok bool
	if byteResponse, ok = response.([]byte); !ok {
		// Response is not a byte array - unmarshal is needed
		var err error
		byteResponse, err = json.Marshal(response)
		assert.NoError(t, err)
	}
	server := httptest.NewServer(createPostHandlerFunc(t, expectedURI, byteResponse, expectedRequestBody,
		expectedStatusCode, expectedHTTPMethod))
	client := buildClient(t, vcsProvider, basicAuth, server)
	return client, server.Close
}

func buildClient(t *testing.T, vcsProvider vcsutils.VcsProvider, basicAuth bool, server *httptest.Server) VcsClient {
	clientBuilder := NewClientBuilder(vcsProvider).ApiEndpoint(server.URL).Token(token)
	if basicAuth {
		clientBuilder = clientBuilder.Username("frogger")
	}
	client, err := clientBuilder.Build()
	assert.NoError(t, err)
	return client
}

func createWaitingServerAndClient(t *testing.T, provider vcsutils.VcsProvider, waitDuration time.Duration) (VcsClient, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if waitDuration > 0 {
			time.Sleep(waitDuration)
		}
		w.WriteHeader(http.StatusOK)
	}))
	clientBuilder := NewClientBuilder(provider).ApiEndpoint(server.URL).Token(token)
	client, err := clientBuilder.Build()
	assert.NoError(t, err)
	return client, server.Close
}

func getAllProviders() []vcsutils.VcsProvider {
	return []vcsutils.VcsProvider{
		vcsutils.GitHub, vcsutils.GitLab, vcsutils.BitbucketServer, vcsutils.BitbucketCloud,
	}
}
