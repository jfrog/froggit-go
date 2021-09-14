package vcsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/jfrog/froggit-go/vcsutils"
	"golang.org/x/oauth2"
)

type BitbucketServerClient struct {
	bitbucketClient *bitbucketv1.DefaultApiService
	logger          *log.Logger
}

func NewBitbucketServerClient(context context.Context, logger *log.Logger, vcsInfo *VcsInfo) (*BitbucketServerClient, error) {
	var httpClient *http.Client
	if vcsInfo.Token != "" {
		httpClient = oauth2.NewClient(context, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: vcsInfo.Token}))
	} else {
		httpClient = &http.Client{}
	}
	bitbucketClient := bitbucketv1.NewAPIClient(context, &bitbucketv1.Configuration{
		HTTPClient: httpClient,
		BasePath:   vcsInfo.ApiEndpoint,
	})
	bitbucketServerClient := &BitbucketServerClient{
		bitbucketClient: bitbucketClient.DefaultApi,
		logger:          logger,
	}
	return bitbucketServerClient, nil
}

func (client *BitbucketServerClient) TestConnection() error {
	_, err := client.bitbucketClient.GetUsers(make(map[string]interface{}))
	return err
}

func (client *BitbucketServerClient) ListRepositories() (map[string][]string, error) {
	projects, err := client.listProjects()
	if err != nil {
		return nil, err
	}

	results := make(map[string][]string)
	for _, project := range projects {
		var apiResponse *bitbucketv1.APIResponse
		for isLastReposPage, nextReposPageStart := true, 0; isLastReposPage; isLastReposPage, nextReposPageStart = bitbucketv1.HasNextPage(apiResponse) {
			// Get all repositories for which the authenticated user has the REPO_READ permission
			apiResponse, err = client.bitbucketClient.GetRepositoriesWithOptions(project, createPaginationOptions(nextReposPageStart))
			if err != nil {
				return nil, err
			}

			repos, err := bitbucketv1.GetRepositoriesResponse(apiResponse)
			if err != nil {
				return nil, err
			}
			for _, repo := range repos {
				results[project] = append(results[project], repo.Slug)
			}
		}
	}
	return results, nil
}

func (client *BitbucketServerClient) ListBranches(owner, repository string) ([]string, error) {
	results := []string{}
	var apiResponse *bitbucketv1.APIResponse
	var err error
	for isLastPage, nextPageStart := true, 0; isLastPage; isLastPage, nextPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = client.bitbucketClient.GetBranches(owner, repository, createPaginationOptions(nextPageStart))
		if err != nil {
			return []string{}, err
		}
		branches, err := bitbucketv1.GetBranchesResponse(apiResponse)
		if err != nil {
			return []string{}, err
		}

		for _, branch := range branches {
			results = append(results, branch.ID)
		}
	}

	return results, nil
}

func (client *BitbucketServerClient) CreateWebhook(owner, repository, branch, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	token := vcsutils.CreateToken()
	hook := createBitbucketServerHook(token, payloadUrl, webhookEvents...)
	response, err := client.bitbucketClient.CreateWebhook(owner, repository, hook, []string{})
	if err != nil {
		return "", "", err
	}
	webhoodId, err := getBitbucketServerWebhookId(response)
	if err != nil {
		return "", "", err
	}
	return webhoodId, token, err
}

func (client *BitbucketServerClient) UpdateWebhook(owner, repository, branch, payloadUrl, token, webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
	webhookIdInt32, err := strconv.ParseInt(webhookId, 10, 32)
	if err != nil {
		return err
	}
	hook := createBitbucketServerHook(token, payloadUrl, webhookEvents...)
	_, err = client.bitbucketClient.UpdateWebhook(owner, repository, int32(webhookIdInt32), hook, []string{})
	return err
}

func (client *BitbucketServerClient) DeleteWebhook(owner, repository, webhookId string) error {
	webhookIdInt32, err := strconv.ParseInt(webhookId, 10, 32)
	if err != nil {
		return err
	}
	_, err = client.bitbucketClient.DeleteWebhook(owner, repository, int32(webhookIdInt32))
	return err
}

func (client *BitbucketServerClient) SetCommitStatus(commitStatus CommitStatus, owner, repository, ref, title, description, detailsUrl string) error {
	_, err := client.bitbucketClient.SetCommitStatus(ref, bitbucketv1.BuildStatus{
		State:       getBitbucketCommitState(commitStatus),
		Key:         title,
		Description: description,
		Url:         detailsUrl,
	})
	return err
}

func (client *BitbucketServerClient) DownloadRepository(owner, repository, branch, localPath string) error {
	response, err := client.bitbucketClient.GetArchive(owner, repository, map[string]interface{}{"format": "tgz"})
	if err != nil {
		return err
	}
	return vcsutils.Untar(localPath, bytes.NewReader(response.Payload), false)
}

func (client BitbucketServerClient) Push(owner, repository string, branch string) error {
	return nil
}

func (client *BitbucketServerClient) CreatePullRequest(owner, repository, sourceBranch, targetBranch, title, description string) error {
	bitbucketRepo := &bitbucketv1.Repository{
		Slug: repository,
		Project: &bitbucketv1.Project{
			Key: owner,
		},
	}
	options := bitbucketv1.PullRequest{
		Title:       title,
		Description: description,
		FromRef: bitbucketv1.PullRequestRef{
			ID:         "refs/heads/" + sourceBranch,
			Repository: *bitbucketRepo,
		},
		ToRef: bitbucketv1.PullRequestRef{
			ID:         "refs/heads/" + targetBranch,
			Repository: *bitbucketRepo,
		},
	}
	_, err := client.bitbucketClient.CreatePullRequest(owner, repository, options)
	return err
}

type projectsResponse struct {
	Values []struct {
		Key string `json:"key,omitempty"`
	} `json:"values,omitempty"`
}

// Get all projects for which the authenticated user has the PROJECT_VIEW permission
func (client *BitbucketServerClient) listProjects() ([]string, error) {
	var apiResponse *bitbucketv1.APIResponse
	var err error
	var projects []string
	for isLastProjectsPage, nextProjectsPageStart := true, 0; isLastProjectsPage; isLastProjectsPage, nextProjectsPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = client.bitbucketClient.GetProjects(createPaginationOptions(nextProjectsPageStart))
		if err != nil {
			return nil, err
		}
		projectsResponse := &projectsResponse{}
		err = unmarshalApiResponseValues(apiResponse, projectsResponse)
		if err != nil {
			return nil, err
		}
		for _, project := range projectsResponse.Values {
			projects = append(projects, project.Key)
		}
	}
	// Add user's private project
	username := apiResponse.Header.Get("X-Ausername")
	if username == "" {
		return []string{}, errors.New("X-Ausername header is missing")
	}
	projects = append(projects, "~"+username)
	return projects, nil
}

func createPaginationOptions(nextPageStart int) map[string]interface{} {
	return map[string]interface{}{"start": nextPageStart}
}

func unmarshalApiResponseValues(response *bitbucketv1.APIResponse, target interface{}) error {
	bytes, err := json.Marshal(response.Values)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &target)
}

func getBitbucketServerWebhookId(r *bitbucketv1.APIResponse) (string, error) {
	webhook := &bitbucketv1.Webhook{}
	err := unmarshalApiResponseValues(r, webhook)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(webhook.ID), nil
}

func createBitbucketServerHook(token, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) *map[string]interface{} {
	return &map[string]interface{}{
		"url":           payloadUrl,
		"configuration": map[string]interface{}{"secret": token},
		"events":        getBitbucketServerWebhookEvent(webhookEvents...),
	}
}

func getBitbucketServerWebhookEvent(webhookEvents ...vcsutils.WebhookEvent) []string {
	events := []string{}
	for _, event := range webhookEvents {
		switch event {
		case vcsutils.PrCreated:
			events = append(events, "pr:opened")
		case vcsutils.PrEdited:
			events = append(events, "pr:from_ref_updated")
		case vcsutils.Push:
			events = append(events, "repo:refs_changed")
		}
	}
	return []string{}
}
