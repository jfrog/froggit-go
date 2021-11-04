package vcsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/ktrysmt/go-bitbucket"
)

type BitbucketCloudClient struct {
	vcsInfo VcsInfo
	url     *url.URL
}

func NewBitbucketCloudClient(vcsInfo VcsInfo) (*BitbucketCloudClient, error) {
	bitbucketClient := &BitbucketCloudClient{
		vcsInfo: vcsInfo,
	}
	if vcsInfo.ApiEndpoint != "" {
		url, err := url.Parse(vcsInfo.ApiEndpoint)
		if err != nil {
			return nil, err
		}
		bitbucketClient.url = url
	}
	return bitbucketClient, nil
}

func (client *BitbucketCloudClient) buildBitbucketCloudClient(_ context.Context) *bitbucket.Client {
	bitbucketClient := bitbucket.NewBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)
	if client.url != nil {
		bitbucketClient.SetApiBaseURL(*client.url)
	}
	return bitbucketClient
}

func (client *BitbucketCloudClient) TestConnection(ctx context.Context) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	_, err := bitbucketClient.User.Profile()
	return err
}

func (client *BitbucketCloudClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	results := make(map[string][]string)
	workspaces, err := bitbucketClient.Workspaces.List()
	if err != nil {
		return nil, err
	}
	for _, workspace := range workspaces.Workspaces {
		repositoriesRes, err := bitbucketClient.Repositories.ListForAccount(&bitbucket.RepositoriesOptions{Owner: workspace.Slug})
		if err != nil {
			return nil, err
		}
		for _, repo := range repositoriesRes.Items {
			results[workspace.Slug] = append(results[workspace.Slug], repo.Slug)
		}
	}
	return results, nil
}

func (client *BitbucketCloudClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	branches, err := bitbucketClient.Repositories.Repository.ListBranches(&bitbucket.RepositoryBranchOptions{Owner: owner, RepoSlug: repository})
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(branches.Branches))
	for _, branch := range branches.Branches {
		results = append(results, branch.Name)
	}
	return results, nil
}

// AddSshKeyToRepository on Bitbucket cloud, the deploy-key is always read-only.
func (client *BitbucketCloudClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, _ Permission) error {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"key name":   keyName,
		"public key": publicKey,
	})
	if err != nil {
		return err
	}
	endpoint := client.vcsInfo.ApiEndpoint
	if endpoint == "" {
		endpoint = bitbucket.DEFAULT_BITBUCKET_API_BASE_URL
	}
	u := fmt.Sprintf("%s/repositories/%s/%s/deploy-keys", endpoint, owner, repository)
	addKeyRequest := bitbucketCloudAddSshKeyRequest{
		Label: keyName,
		Key:   publicKey,
	}

	body := new(bytes.Buffer)
	err = json.NewEncoder(body).Encode(addKeyRequest)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)

	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	response, err := bitbucketClient.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = vcsutils.DiscardResponseBody(response)
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 300 {
		return fmt.Errorf(response.Status)
	}
	return nil
}

type bitbucketCloudAddSshKeyRequest struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

func (client *BitbucketCloudClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadUrl string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	token := vcsutils.CreateToken()
	options := &bitbucket.WebhooksOptions{
		Active:   true,
		Owner:    owner,
		RepoSlug: repository,
		Url:      payloadUrl + "?token=" + url.QueryEscape(token),
		Events:   getBitbucketCloudWebhookEvents(webhookEvents...),
	}
	response, err := bitbucketClient.Repositories.Webhooks.Create(options)
	if err != nil {
		return "", "", err
	}
	id, err := getBitbucketCloudWebhookId(response)
	if err != nil {
		return "", "", err
	}
	return id, token, err
}

func (client *BitbucketCloudClient) UpdateWebhook(ctx context.Context, owner, repository, _, payloadUrl, token,
	webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	options := &bitbucket.WebhooksOptions{
		Active:   true,
		Uuid:     webhookId,
		Owner:    owner,
		RepoSlug: repository,
		Url:      payloadUrl + "?token=" + url.QueryEscape(token),
		Events:   getBitbucketCloudWebhookEvents(webhookEvents...),
	}
	_, err := bitbucketClient.Repositories.Webhooks.Update(options)
	return err
}

func (client *BitbucketCloudClient) DeleteWebhook(ctx context.Context, owner, repository, webhookId string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	options := &bitbucket.WebhooksOptions{
		Uuid:     webhookId,
		Owner:    owner,
		RepoSlug: repository,
	}
	_, err := bitbucketClient.Repositories.Webhooks.Delete(options)
	return err
}

func (client *BitbucketCloudClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository,
	ref, title, description, detailsUrl string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	commitOptions := &bitbucket.CommitsOptions{
		Owner:    owner,
		RepoSlug: repository,
		Revision: ref,
	}
	commitStatusOptions := &bitbucket.CommitStatusOptions{
		State:       getBitbucketCommitState(commitStatus),
		Key:         title,
		Description: description,
		Url:         detailsUrl,
	}
	_, err := bitbucketClient.Repositories.Commits.CreateCommitStatus(commitOptions, commitStatusOptions)
	return err
}

func (client *BitbucketCloudClient) DownloadRepository(ctx context.Context, owner, repository, branch,
	localPath string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	repo, err := bitbucketClient.Repositories.Repository.Get(&bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repository,
	})
	if err != nil {
		return err
	}

	downloadLink, err := getDownloadLink(repo, branch)
	if err != nil {
		return err
	}

	getRequest, err := http.NewRequestWithContext(ctx, "GET", downloadLink, nil)
	if err != nil {
		return err
	}
	if len(client.vcsInfo.Username) > 0 || len(client.vcsInfo.Token) > 0 {
		getRequest.SetBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)
	}

	response, err := bitbucketClient.HttpClient.Do(getRequest)
	if err != nil {
		return err
	}
	return vcsutils.Untar(localPath, response.Body, true)
}

func (client *BitbucketCloudClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch,
	targetBranch, title, description string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	options := &bitbucket.PullRequestsOptions{
		Owner:             owner,
		SourceRepository:  owner + "/" + repository,
		RepoSlug:          repository,
		SourceBranch:      sourceBranch,
		DestinationBranch: targetBranch,
		Title:             title,
		Description:       description,
	}
	_, err := bitbucketClient.Repositories.PullRequests.Create(options)
	return err
}

func (client *BitbucketCloudClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"branch":     branch,
	})
	if err != nil {
		return CommitInfo{}, err
	}
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	bitbucketClient.Pagelen = 1
	options := &bitbucket.CommitsOptions{
		Owner:       owner,
		RepoSlug:    repository,
		Branchortag: branch,
	}
	commits, err := bitbucketClient.Repositories.Commits.GetCommits(options)
	if err != nil {
		return CommitInfo{}, err
	}
	parsedCommits, err := extractCommitFromResponse(commits)
	if err != nil {
		return CommitInfo{}, err
	}
	if len(parsedCommits.Values) > 0 {
		latestCommit := parsedCommits.Values[0]
		parents := make([]string, len(latestCommit.Parents))
		for i, p := range latestCommit.Parents {
			parents[i] = p.Hash
		}
		return CommitInfo{
			Hash:          latestCommit.Hash,
			AuthorName:    latestCommit.Author.User.DisplayName,
			CommitterName: "", // not provided
			Url:           latestCommit.Links.Self.Href,
			Timestamp:     latestCommit.Date.UTC().Unix(),
			Message:       latestCommit.Message,
			ParentHashes:  parents,
		}, nil
	}
	return CommitInfo{}, nil
}

func extractCommitFromResponse(commits interface{}) (*commitResponse, error) {
	var res commitResponse
	b, err := json.Marshal(commits)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

type commitResponse struct {
	Values []commitDetails `json:"values"`
}

type commitDetails struct {
	Hash    string    `json:"hash"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
	Author  struct {
		User struct {
			DisplayName string `json:"display_name"`
		} `json:"user"`
	} `json:"author"`
	Links struct {
		Self link `json:"self"`
	} `json:"links"`
	Parents []struct {
		Hash string `json:"hash"`
	} `json:"parents"`
}

type link struct {
	Href string `json:"href"`
}

// Extract the webhook id from the webhook create response
func getBitbucketCloudWebhookId(r interface{}) (string, error) {
	webhook := &bitbucket.WebhooksOptions{}
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(b, &webhook)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(strings.TrimLeft(webhook.Uuid, "{"), "}"), nil
}

// Get varargs of webhook events and return a slice of Bitbucket cloud webhook events
func getBitbucketCloudWebhookEvents(webhookEvents ...vcsutils.WebhookEvent) []string {
	events := make([]string, 0, len(webhookEvents))
	for _, event := range webhookEvents {
		switch event {
		case vcsutils.PrCreated:
			events = append(events, "pullrequest:created")
		case vcsutils.PrEdited:
			events = append(events, "pullrequest:updated")
		case vcsutils.Push:
			events = append(events, "repo:push")
		}
	}
	return events
}

// The get repository request returns HTTP link to the repository - extract the link from the response.
func getDownloadLink(repo *bitbucket.Repository, branch string) (string, error) {
	repositoryHtmlLinks := &link{}
	b, err := json.Marshal(repo.Links["html"])
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(b, repositoryHtmlLinks)
	if err != nil {
		return "", err
	}
	htmlLink := repositoryHtmlLinks.Href
	if htmlLink == "" {
		return "", fmt.Errorf("couldn't find repository HTML link: %s", repo.Links["html"])
	}
	return htmlLink + "/get/" + branch + ".tar.gz", err
}
