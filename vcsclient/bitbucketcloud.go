package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/ktrysmt/go-bitbucket"
)

type BitbucketCloudClient struct {
	bitbucketClient *bitbucket.Client
	username        string
	token           string
}

func NewBitbucketCloudClient(vcsInfo *VcsInfo) (*BitbucketCloudClient, error) {
	err := os.Setenv("BITBUCKET_API_BASE_URL", vcsInfo.ApiEndpoint)
	if err != nil {
		return nil, err
	}
	bitbucketClient := &BitbucketCloudClient{
		bitbucketClient: bitbucket.NewBasicAuth(vcsInfo.Username, vcsInfo.Token),
		username:        vcsInfo.Username,
		token:           vcsInfo.Token,
	}
	return bitbucketClient, nil
}

func (client *BitbucketCloudClient) TestConnection(_ context.Context) error {
	_, err := client.bitbucketClient.User.Profile()
	return err
}

func (client *BitbucketCloudClient) ListRepositories(_ context.Context) (map[string][]string, error) {
	results := make(map[string][]string)
	workspaces, err := client.bitbucketClient.Workspaces.List()
	if err != nil {
		return nil, err
	}
	for _, workspace := range workspaces.Workspaces {
		repositoriesRes, err := client.bitbucketClient.Repositories.ListForAccount(&bitbucket.RepositoriesOptions{Owner: workspace.Slug})
		if err != nil {
			return nil, err
		}
		for _, repo := range repositoriesRes.Items {
			results[workspace.Slug] = append(results[workspace.Slug], repo.Slug)
		}
	}
	return results, nil
}

func (client *BitbucketCloudClient) ListBranches(_ context.Context, owner, repository string) ([]string, error) {
	branches, err := client.bitbucketClient.Repositories.Repository.ListBranches(&bitbucket.RepositoryBranchOptions{Owner: owner, RepoSlug: repository})
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(branches.Branches))
	for _, branch := range branches.Branches {
		results = append(results, branch.Name)
	}
	return results, nil
}

func (client *BitbucketCloudClient) CreateWebhook(_ context.Context, owner, repository, _, payloadUrl string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	token := vcsutils.CreateToken()
	options := &bitbucket.WebhooksOptions{
		Active:   true,
		Owner:    owner,
		RepoSlug: repository,
		Url:      payloadUrl + "?token=" + url.QueryEscape(token),
		Events:   getBitbucketCloudWebhookEvents(webhookEvents...),
	}
	response, err := client.bitbucketClient.Repositories.Webhooks.Create(options)
	if err != nil {
		return "", "", err
	}
	id, err := getBitbucketCloudWebhookId(response)
	if err != nil {
		return "", "", err
	}
	return id, token, err
}

func (client *BitbucketCloudClient) UpdateWebhook(_ context.Context, owner, repository, _, payloadUrl, token,
	webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
	options := &bitbucket.WebhooksOptions{
		Active:   true,
		Uuid:     webhookId,
		Owner:    owner,
		RepoSlug: repository,
		Url:      payloadUrl + "?token=" + url.QueryEscape(token),
		Events:   getBitbucketCloudWebhookEvents(webhookEvents...),
	}
	_, err := client.bitbucketClient.Repositories.Webhooks.Update(options)
	return err
}

func (client *BitbucketCloudClient) DeleteWebhook(_ context.Context, owner, repository, webhookId string) error {
	options := &bitbucket.WebhooksOptions{
		Uuid:     webhookId,
		Owner:    owner,
		RepoSlug: repository,
	}
	_, err := client.bitbucketClient.Repositories.Webhooks.Delete(options)
	return err
}

func (client *BitbucketCloudClient) SetCommitStatus(_ context.Context, commitStatus CommitStatus, owner, repository,
	ref, title, description, detailsUrl string) error {
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
	_, err := client.bitbucketClient.Repositories.Commits.CreateCommitStatus(commitOptions, commitStatusOptions)
	return err
}

func (client *BitbucketCloudClient) DownloadRepository(_ context.Context, owner, repository, branch,
	localPath string) error {
	repo, err := client.bitbucketClient.Repositories.Repository.Get(&bitbucket.RepositoryOptions{
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

	getRequest, err := http.NewRequest("GET", downloadLink, nil)
	if err != nil {
		return err
	}
	if len(client.username) > 0 || len(client.token) > 0 {
		getRequest.SetBasicAuth(client.username, client.token)
	}

	response, err := client.bitbucketClient.HttpClient.Do(getRequest)
	if err != nil {
		return err
	}
	return vcsutils.Untar(localPath, response.Body, true)
}

func (client *BitbucketCloudClient) CreatePullRequest(_ context.Context, owner, repository, sourceBranch,
	targetBranch, title, description string) error {
	options := &bitbucket.PullRequestsOptions{
		Owner:             owner,
		SourceRepository:  owner + "/" + repository,
		RepoSlug:          repository,
		SourceBranch:      sourceBranch,
		DestinationBranch: targetBranch,
		Title:             title,
		Description:       description,
	}
	_, err := client.bitbucketClient.Repositories.PullRequests.Create(options)
	return err
}

// Extract the webhook id from the webhook create response
func getBitbucketCloudWebhookId(r interface{}) (string, error) {
	webhook := &bitbucket.WebhooksOptions{}
	bytes, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(bytes, &webhook)
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
	repositoryHtmlLinks := &repositoryHtmlLinks{}
	bytes, err := json.Marshal(repo.Links["html"])
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(bytes, repositoryHtmlLinks)
	if err != nil {
		return "", err
	}
	htmlLink := repositoryHtmlLinks.Href
	if htmlLink == "" {
		return "", fmt.Errorf("couldn't find repository HTML link: %s", repo.Links["html"])
	}
	return htmlLink + "/get/" + branch + ".tar.gz", err
}

type repositoryHtmlLinks struct {
	Href string `json:"href,omitempty"`
}
