package vcsclient

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-github/v38/github"
	"github.com/jfrog/froggit-go/vcsutils"
	"golang.org/x/oauth2"
)

const scopesHeader = "X-OAuth-Scopes"

type GitHubClient struct {
	ghClient *github.Client
	logger   *log.Logger
	context  context.Context
}

func NewGitHubClient(context context.Context, logger *log.Logger, vcsInfo *VcsInfo) (*GitHubClient, error) {
	httpClient := &http.Client{}
	if vcsInfo.Token != "" {
		httpClient = oauth2.NewClient(context, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: vcsInfo.Token}))
	}
	client := github.NewClient(httpClient)
	if vcsInfo.ApiEndpoint != "" {
		url, err := url.Parse(strings.TrimSuffix(vcsInfo.ApiEndpoint, "/") + "/")
		if err != nil {
			return nil, nil
		}
		client.BaseURL = url
	}
	return &GitHubClient{
		ghClient: client,
		logger:   logger,
		context:  context,
	}, nil
}

func (client *GitHubClient) TestConnection() error {
	_, _, err := client.ghClient.Zen(client.context)
	return err
}

func (client *GitHubClient) ListRepositories() (map[string][]string, error) {
	results := make(map[string][]string)
	for nextPage := 0; ; nextPage++ {
		options := &github.RepositoryListOptions{ListOptions: github.ListOptions{Page: nextPage}}
		repos, response, err := client.ghClient.Repositories.List(client.context, "", options)
		if err != nil {
			return nil, err
		}
		for _, repo := range repos {
			results[*repo.Owner.Login] = append(results[*repo.Owner.Login], *repo.Name)
		}
		if nextPage+1 >= response.LastPage {
			break
		}
	}
	return results, nil
}

func (client *GitHubClient) ListBranches(owner, repository string) ([]string, error) {
	branches, _, err := client.ghClient.Repositories.ListBranches(client.context, owner, repository, nil)
	if err != nil {
		return []string{}, err
	}

	results := []string{}
	for _, repo := range branches {
		results = append(results, *repo.Name)
	}
	return results, nil
}

func (client *GitHubClient) CreateWebhook(owner, repository, branch, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	token := vcsutils.CreateToken()
	hook := createGitHubHook(token, payloadUrl, webhookEvents...)
	responseHook, _, err := client.ghClient.Repositories.CreateHook(client.context, owner, repository, hook)
	if err != nil {
		return "", "", err
	}
	return strconv.FormatInt(*responseHook.ID, 10), token, err
}

func (client *GitHubClient) UpdateWebhook(owner, repository, branch, payloadUrl, token, webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
	webhookIdInt64, err := strconv.ParseInt(webhookId, 10, 64)
	if err != nil {
		return err
	}
	hook := createGitHubHook(token, payloadUrl, webhookEvents...)
	_, _, err = client.ghClient.Repositories.EditHook(client.context, owner, repository, webhookIdInt64, hook)
	return err
}

func (client *GitHubClient) DeleteWebhook(owner, repository, webhookId string) error {
	webhookIdInt64, err := strconv.ParseInt(webhookId, 10, 64)
	if err != nil {
		return err
	}
	_, err = client.ghClient.Repositories.DeleteHook(client.context, owner, repository, webhookIdInt64)
	return err
}

func (client *GitHubClient) SetCommitStatus(commitStatus CommitStatus, owner, repository, ref, title, description, detailsUrl string) error {
	state := getGitHubCommitState(commitStatus)
	status := &github.RepoStatus{
		Context:     &title,
		TargetURL:   &detailsUrl,
		State:       &state,
		Description: &description,
	}
	_, _, err := client.ghClient.Repositories.CreateStatus(client.context, owner, repository, ref, status)
	return err
}

func (client *GitHubClient) DownloadRepository(owner, repository, branch, localPath string) error {
	url, _, err := client.ghClient.Repositories.GetArchiveLink(client.context, owner, repository, github.Tarball, &github.RepositoryContentGetOptions{Ref: branch}, true)
	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", url.String(), nil)
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return vcsutils.Untar(localPath, resp.Body, true)
}

func (client GitHubClient) Push(owner, repository string, branch string) error {
	return nil
}

func (client *GitHubClient) CreatePullRequest(owner, repository, sourceBranch, targetBranch, title, description string) error {
	head := owner + ":" + sourceBranch
	_, _, err := client.ghClient.PullRequests.Create(client.context, owner, repository, &github.NewPullRequest{
		Title: &title,
		Body:  &description,
		Head:  &head,
		Base:  &targetBranch,
	})
	return err
}

func createGitHubHook(token, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) *github.Hook {
	return &github.Hook{
		Events: getWebhookEvents(webhookEvents...),
		Config: map[string]interface{}{
			"url":          payloadUrl,
			"content_type": "json",
			"secret":       token,
		},
	}
}

func getWebhookEvents(webhookEvents ...vcsutils.WebhookEvent) []string {
	events := []string{}
	for _, event := range webhookEvents {
		switch event {
		case vcsutils.PrCreated, vcsutils.PrEdited:
			events = append(events, "pull_request")
		case vcsutils.Push:
			events = append(events, "push")
		}
	}
	return events
}

func getGitHubCommitState(commitState CommitStatus) string {
	switch commitState {
	case Pass:
		return "success"
	case Fail:
		return "failure"
	case Error:
		return "error"
	case InProgress:
		return "pending"
	}
	return ""
}
