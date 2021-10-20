package vcsclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-github/v38/github"
	"github.com/jfrog/froggit-go/vcsutils"
	"golang.org/x/oauth2"
)

type GitHubClient struct {
	vcsInfo *VcsInfo
}

func NewGitHubClient(vcsInfo *VcsInfo) (*GitHubClient, error) {
	return &GitHubClient{vcsInfo: vcsInfo}, nil
}

func (client *GitHubClient) TestConnection(ctx context.Context) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	_, _, err = ghClient.Zen(ctx)
	return err
}

func (client *GitHubClient) buildGithubClient(ctx context.Context) (*github.Client, error) {
	httpClient := &http.Client{}
	if client.vcsInfo.Token != "" {
		httpClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: client.vcsInfo.Token}))
	}
	ghClient := github.NewClient(httpClient)
	if client.vcsInfo.ApiEndpoint != "" {
		baseUrl, err := url.Parse(strings.TrimSuffix(client.vcsInfo.ApiEndpoint, "/") + "/")
		if err != nil {
			return nil, err
		}
		ghClient.BaseURL = baseUrl
	}
	return ghClient, nil
}

func (client *GitHubClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}
	results := make(map[string][]string)
	for nextPage := 0; ; nextPage++ {
		options := &github.RepositoryListOptions{ListOptions: github.ListOptions{Page: nextPage}}
		repos, response, err := ghClient.Repositories.List(ctx, "", options)
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

func (client *GitHubClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}
	branches, _, err := ghClient.Repositories.ListBranches(ctx, owner, repository, nil)
	if err != nil {
		return []string{}, err
	}

	results := make([]string, 0, len(branches))
	for _, repo := range branches {
		results = append(results, *repo.Name)
	}
	return results, nil
}

func (client *GitHubClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadUrl string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return "", "", err
	}
	token := vcsutils.CreateToken()
	hook := createGitHubHook(token, payloadUrl, webhookEvents...)
	responseHook, _, err := ghClient.Repositories.CreateHook(ctx, owner, repository, hook)
	if err != nil {
		return "", "", err
	}
	return strconv.FormatInt(*responseHook.ID, 10), token, err
}

func (client *GitHubClient) UpdateWebhook(ctx context.Context, owner, repository, _, payloadUrl, token,
	webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	webhookIdInt64, err := strconv.ParseInt(webhookId, 10, 64)
	if err != nil {
		return err
	}
	hook := createGitHubHook(token, payloadUrl, webhookEvents...)
	_, _, err = ghClient.Repositories.EditHook(ctx, owner, repository, webhookIdInt64, hook)
	return err
}

func (client *GitHubClient) DeleteWebhook(ctx context.Context, owner, repository, webhookId string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	webhookIdInt64, err := strconv.ParseInt(webhookId, 10, 64)
	if err != nil {
		return err
	}
	_, err = ghClient.Repositories.DeleteHook(ctx, owner, repository, webhookIdInt64)
	return err
}

func (client *GitHubClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref,
	title, description, detailsUrl string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	state := getGitHubCommitState(commitStatus)
	status := &github.RepoStatus{
		Context:     &title,
		TargetURL:   &detailsUrl,
		State:       &state,
		Description: &description,
	}
	_, _, err = ghClient.Repositories.CreateStatus(ctx, owner, repository, ref, status)
	return err
}

func (client *GitHubClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	baseUrl, _, err := ghClient.Repositories.GetArchiveLink(ctx, owner, repository, github.Tarball,
		&github.RepositoryContentGetOptions{Ref: branch}, true)
	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", baseUrl.String(), nil)
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return vcsutils.Untar(localPath, resp.Body, true)
}

func (client *GitHubClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch,
	title, description string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	head := owner + ":" + sourceBranch
	_, _, err = ghClient.PullRequests.Create(ctx, owner, repository, &github.NewPullRequest{
		Title: &title,
		Body:  &description,
		Head:  &head,
		Base:  &targetBranch,
	})
	return err
}

func createGitHubHook(token, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) *github.Hook {
	return &github.Hook{
		Events: getGitHubWebhookEvents(webhookEvents...),
		Config: map[string]interface{}{
			"url":          payloadUrl,
			"content_type": "json",
			"secret":       token,
		},
	}
}

// Get varargs of webhook events and return a slice of GitHub webhook events
func getGitHubWebhookEvents(webhookEvents ...vcsutils.WebhookEvent) []string {
	events := make([]string, 0, len(webhookEvents))
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
