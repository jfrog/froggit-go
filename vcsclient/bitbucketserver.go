package vcsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/jfrog/froggit-go/vcsutils"
	"golang.org/x/oauth2"
	"net/http"
	"strconv"
)

type BitbucketServerClient struct {
	vcsInfo VcsInfo
}

func NewBitbucketServerClient(vcsInfo VcsInfo) (*BitbucketServerClient, error) {
	bitbucketServerClient := &BitbucketServerClient{
		vcsInfo: vcsInfo,
	}
	return bitbucketServerClient, nil
}

func (client *BitbucketServerClient) buildBitbucketClient(ctx context.Context) (*bitbucketv1.DefaultApiService, error) {
	httpClient := &http.Client{}
	if client.vcsInfo.Token != "" {
		httpClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: client.vcsInfo.Token}))
	}
	bbClient := bitbucketv1.NewAPIClient(ctx, &bitbucketv1.Configuration{
		HTTPClient: httpClient,
		BasePath:   client.vcsInfo.ApiEndpoint,
	})
	return bbClient.DefaultApi, nil
}

func (client *BitbucketServerClient) TestConnection(ctx context.Context) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}

	options := map[string]interface{}{"limit": 1}
	_, err = bitbucketClient.GetUsers(options)
	return err
}

func (client *BitbucketServerClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := client.listProjects(bitbucketClient)
	if err != nil {
		return nil, err
	}

	results := make(map[string][]string)
	for _, project := range projects {
		var apiResponse *bitbucketv1.APIResponse
		for isLastReposPage, nextReposPageStart := true, 0; isLastReposPage; isLastReposPage, nextReposPageStart = bitbucketv1.HasNextPage(apiResponse) {
			// Get all repositories for which the authenticated user has the REPO_READ permission
			apiResponse, err = bitbucketClient.GetRepositoriesWithOptions(project, createPaginationOptions(nextReposPageStart))
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

func (client *BitbucketServerClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}
	var results []string
	var apiResponse *bitbucketv1.APIResponse
	for isLastPage, nextPageStart := true, 0; isLastPage; isLastPage, nextPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = bitbucketClient.GetBranches(owner, repository, createPaginationOptions(nextPageStart))
		if err != nil {
			return nil, err
		}
		branches, err := bitbucketv1.GetBranchesResponse(apiResponse)
		if err != nil {
			return nil, err
		}

		for _, branch := range branches {
			results = append(results, branch.ID)
		}
	}

	return results, nil
}

func (client *BitbucketServerClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadUrl string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return "", "", err
	}
	token := vcsutils.CreateToken()
	hook := createBitbucketServerHook(token, payloadUrl, webhookEvents...)
	response, err := bitbucketClient.CreateWebhook(owner, repository, hook, []string{})
	if err != nil {
		return "", "", err
	}
	webhoodId, err := getBitbucketServerWebhookId(response)
	if err != nil {
		return "", "", err
	}
	return webhoodId, token, err
}

func (client *BitbucketServerClient) UpdateWebhook(ctx context.Context, owner, repository, _, payloadUrl, token,
	webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	webhookIdInt32, err := strconv.ParseInt(webhookId, 10, 32)
	if err != nil {
		return err
	}
	hook := createBitbucketServerHook(token, payloadUrl, webhookEvents...)
	_, err = bitbucketClient.UpdateWebhook(owner, repository, int32(webhookIdInt32), hook, []string{})
	return err
}

func (client *BitbucketServerClient) DeleteWebhook(ctx context.Context, owner, repository, webhookId string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	webhookIdInt32, err := strconv.ParseInt(webhookId, 10, 32)
	if err != nil {
		return err
	}
	_, err = bitbucketClient.DeleteWebhook(owner, repository, int32(webhookIdInt32))
	return err
}

func (client *BitbucketServerClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, _, _, ref, title,
	description, detailsUrl string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	_, err = bitbucketClient.SetCommitStatus(ref, bitbucketv1.BuildStatus{
		State:       getBitbucketCommitState(commitStatus),
		Key:         title,
		Description: description,
		Url:         detailsUrl,
	})
	return err
}

func (client *BitbucketServerClient) DownloadRepository(ctx context.Context, owner, repository, _, localPath string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	response, err := bitbucketClient.GetArchive(owner, repository, map[string]interface{}{"format": "tgz"})
	if err != nil {
		return err
	}
	return vcsutils.Untar(localPath, bytes.NewReader(response.Payload), false)
}

func (client *BitbucketServerClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch,
	title, description string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
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
	_, err = bitbucketClient.CreatePullRequest(owner, repository, options)
	return err
}

type projectsResponse struct {
	Values []struct {
		Key string `json:"key,omitempty"`
	} `json:"values,omitempty"`
}

func (client *BitbucketServerClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	err := validateParametersNotBlank(owner, repository, branch)
	if err != nil {
		return CommitInfo{}, err
	}

	options := map[string]interface{}{
		"limit": 1,
		"until": branch,
	}
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}

	apiResponse, err := bitbucketClient.GetCommits(owner, repository, options)
	if err != nil {
		return CommitInfo{}, err
	}
	commits, err := bitbucketv1.GetCommitsResponse(apiResponse)
	if err != nil {
		return CommitInfo{}, err
	}
	if len(commits) > 0 {
		latestCommit := commits[0]
		parents := make([]string, len(latestCommit.Parents))
		for i, p := range latestCommit.Parents {
			parents[i] = p.ID
		}
		return CommitInfo{
			Hash:          latestCommit.ID,
			AuthorName:    latestCommit.Author.Name,
			CommitterName: latestCommit.Committer.Name,
			Url:           "", // URL not provided
			Timestamp:     latestCommit.CommitterTimestamp,
			Message:       latestCommit.Message,
			ParentHashes:  parents,
		}, nil
	}
	return CommitInfo{}, nil
}

// Get all projects for which the authenticated user has the PROJECT_VIEW permission
func (client *BitbucketServerClient) listProjects(bitbucketClient *bitbucketv1.DefaultApiService) ([]string, error) {
	var apiResponse *bitbucketv1.APIResponse
	var err error
	var projects []string
	for isLastProjectsPage, nextProjectsPageStart := true, 0; isLastProjectsPage; isLastProjectsPage, nextProjectsPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = bitbucketClient.GetProjects(createPaginationOptions(nextProjectsPageStart))
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
	responseBytes, err := json.Marshal(response.Values)
	if err != nil {
		return err
	}
	return json.Unmarshal(responseBytes, &target)
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
		"events":        getBitbucketServerWebhookEvents(webhookEvents...),
	}
}

// Get varargs of webhook events and return a slice of Bitbucket server webhook events
func getBitbucketServerWebhookEvents(webhookEvents ...vcsutils.WebhookEvent) []string {
	events := make([]string, 0, len(webhookEvents))
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
	return events
}
