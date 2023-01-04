package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v45/github"
	"github.com/grokify/mogo/encoding/base64"
	"github.com/jfrog/froggit-go/vcsutils"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// GitHubClient API version 3
type GitHubClient struct {
	vcsInfo VcsInfo
	logger  Log
}

// NewGitHubClient create a new GitHubClient
func NewGitHubClient(vcsInfo VcsInfo, logger Log) (*GitHubClient, error) {
	return &GitHubClient{vcsInfo: vcsInfo, logger: logger}, nil
}

// TestConnection on GitHub
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
	if client.vcsInfo.APIEndpoint != "" {
		baseURL, err := url.Parse(strings.TrimSuffix(client.vcsInfo.APIEndpoint, "/") + "/")
		if err != nil {
			return nil, err
		}
		ghClient.BaseURL = baseURL
	}
	return ghClient, nil
}

// AddSshKeyToRepository on GitHub
func (client *GitHubClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission Permission) error {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"key name":   keyName,
		"public key": publicKey,
	})
	if err != nil {
		return err
	}
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}

	readOnly := true
	if permission == ReadWrite {
		readOnly = false
	}
	key := github.Key{
		Key:      &publicKey,
		Title:    &keyName,
		ReadOnly: &readOnly,
	}
	_, _, err = ghClient.Repositories.CreateKey(ctx, owner, repository, &key)
	return err
}

// ListRepositories on GitHub
func (client *GitHubClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}
	results := make(map[string][]string)
	for nextPage := 1; ; nextPage++ {
		options := &github.RepositoryListOptions{ListOptions: github.ListOptions{Page: nextPage}}
		repos, response, err := ghClient.Repositories.List(ctx, "", options)
		if err != nil {
			return nil, err
		}
		for _, repo := range repos {
			results[*repo.Owner.Login] = append(results[*repo.Owner.Login], *repo.Name)
		}
		if nextPage+1 > response.LastPage {
			break
		}
	}
	return results, nil
}

// ListBranches on GitHub
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

// CreateWebhook on GitHub
func (client *GitHubClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadURL string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return "", "", err
	}
	token := vcsutils.CreateToken()
	hook := createGitHubHook(token, payloadURL, webhookEvents...)
	responseHook, _, err := ghClient.Repositories.CreateHook(ctx, owner, repository, hook)
	if err != nil {
		return "", "", err
	}
	return strconv.FormatInt(*responseHook.ID, 10), token, err
}

// UpdateWebhook on GitHub
func (client *GitHubClient) UpdateWebhook(ctx context.Context, owner, repository, _, payloadURL, token,
	webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	webhookIDInt64, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		return err
	}
	hook := createGitHubHook(token, payloadURL, webhookEvents...)
	_, _, err = ghClient.Repositories.EditHook(ctx, owner, repository, webhookIDInt64, hook)
	return err
}

// DeleteWebhook on GitHub
func (client *GitHubClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	webhookIDInt64, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		return err
	}
	_, err = ghClient.Repositories.DeleteHook(ctx, owner, repository, webhookIDInt64)
	return err
}

// SetCommitStatus on GitHub
func (client *GitHubClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref,
	title, description, detailsURL string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	state := getGitHubCommitState(commitStatus)
	status := &github.RepoStatus{
		Context:     &title,
		TargetURL:   &detailsURL,
		State:       &state,
		Description: &description,
	}
	_, _, err = ghClient.Repositories.CreateStatus(ctx, owner, repository, ref, status)
	return err
}

// DownloadRepository on GitHub
func (client *GitHubClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	client.logger.Debug("getting GitHub archive link to download")
	baseURL, _, err := ghClient.Repositories.GetArchiveLink(ctx, owner, repository, github.Tarball,
		&github.RepositoryContentGetOptions{Ref: branch}, true)
	if err != nil {
		return err
	}

	client.logger.Debug("received archive url:", baseURL.String())
	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", baseURL.String(), nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err = vcsutils.CheckResponseStatusWithBody(resp, http.StatusOK); err != nil {
		return err
	}
	client.logger.Info(repository, "downloaded successfully, starting with repository extraction")
	err = vcsutils.Untar(localPath, resp.Body, true)
	if err != nil {
		return err
	}
	client.logger.Info("extracted repository successfully")
	return vcsutils.CreateDotGitFolderWithRemote(localPath, "origin",
		fmt.Sprintf("https://github.com/%s/%s.git", owner, repository))
}

// CreatePullRequest on GitHub
func (client *GitHubClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch,
	title, description string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	head := owner + ":" + sourceBranch
	client.logger.Debug("creating new pull request:", title)
	_, _, err = ghClient.PullRequests.Create(ctx, owner, repository, &github.NewPullRequest{
		Title: &title,
		Body:  &description,
		Head:  &head,
		Base:  &targetBranch,
	})
	return err
}

// ListOpenPullRequests on GitHub
func (client *GitHubClient) ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}
	client.logger.Debug("fetching open pull requests in", repository)
	pullRequests, _, err := ghClient.PullRequests.List(ctx, owner, repository, &github.PullRequestListOptions{
		State: "open",
	})
	if err != nil {
		return []PullRequestInfo{}, err
	}
	return mapGitHubPullRequestToPullRequestInfoList(pullRequests)
}

// AddPullRequestComment on GitHub
func (client *GitHubClient) AddPullRequestComment(ctx context.Context, owner, repository, content string, pullRequestID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "content": content})
	if err != nil {
		return err
	}
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	// We use the Issues API to add a regular comment. The PullRequests API adds a code review comment.
	_, _, err = ghClient.Issues.CreateComment(ctx, owner, repository, pullRequestID, &github.IssueComment{
		Body: &content,
	})
	return err
}

// ListPullRequestComments on GitHub
func (client *GitHubClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return []CommentInfo{}, err
	}
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return []CommentInfo{}, err
	}
	commentsList, _, err := ghClient.Issues.ListComments(ctx, owner, repository, pullRequestID, &github.IssueListCommentsOptions{})
	if err != nil {
		return []CommentInfo{}, err
	}
	return mapGitHubCommentToCommentInfoList(commentsList)
}

// GetLatestCommit on GitHub
func (client *GitHubClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"branch":     branch,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}
	listOptions := &github.CommitsListOptions{
		SHA: branch,
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 1,
		},
	}
	commits, _, err := ghClient.Repositories.ListCommits(ctx, owner, repository, listOptions)
	if err != nil {
		return CommitInfo{}, err
	}
	if len(commits) > 0 {
		latestCommit := commits[0]
		return mapGitHubCommitToCommitInfo(latestCommit), nil
	}
	return CommitInfo{}, nil
}

// GetRepositoryInfo on GitHub
func (client *GitHubClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return RepositoryInfo{}, err
	}

	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return RepositoryInfo{}, err
	}

	repo, _, err := ghClient.Repositories.Get(ctx, owner, repository)
	if err != nil {
		return RepositoryInfo{}, err
	}
	return RepositoryInfo{CloneInfo: CloneInfo{HTTP: repo.GetCloneURL(), SSH: repo.GetSSHURL()}}, nil
}

// GetCommitBySha on GitHub
func (client *GitHubClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"sha":        sha,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}

	commit, _, err := ghClient.Repositories.GetCommit(ctx, owner, repository, sha, nil)
	if err != nil {
		return CommitInfo{}, err
	}

	return mapGitHubCommitToCommitInfo(commit), nil
}

// CreateLabel on GitHub
func (client *GitHubClient) CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "LabelInfo.name": labelInfo.Name})
	if err != nil {
		return err
	}

	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}

	_, _, err = ghClient.Issues.CreateLabel(ctx, owner, repository, &github.Label{
		Name:        &labelInfo.Name,
		Description: &labelInfo.Description,
		Color:       &labelInfo.Color,
	})

	return err
}

// GetLabel on GitHub
func (client *GitHubClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "name": name})
	if err != nil {
		return nil, err
	}

	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}

	label, response, err := ghClient.Issues.GetLabel(ctx, owner, repository, name)
	if err != nil {
		if response.Response.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &LabelInfo{
		Name:        *label.Name,
		Description: *label.Description,
		Color:       *label.Color,
	}, err
}

// ListPullRequestLabels on GitHub
func (client *GitHubClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return []string{}, err
	}

	results := []string{}
	for nextPage := 0; ; nextPage++ {
		options := &github.ListOptions{Page: nextPage}
		labels, response, err := ghClient.Issues.ListLabelsByIssue(ctx, owner, repository, pullRequestID, options)
		if err != nil {
			return []string{}, err
		}
		for _, label := range labels {
			results = append(results, *label.Name)
		}
		if nextPage+1 >= response.LastPage {
			break
		}
	}
	return results, nil
}

// UnlabelPullRequest on GitHub
func (client *GitHubClient) UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return err
	}
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}

	_, err = ghClient.Issues.RemoveLabelForIssue(ctx, owner, repository, pullRequestID, name)
	return err
}

// UploadCodeScanning to GitHub Security tab
func (client *GitHubClient) UploadCodeScanning(ctx context.Context, owner, repository, branch, scanResults string) (string, error) {
	packagedScan, err := packScanningResult(scanResults)
	if err != nil {
		return "", err
	}
	commit, err := client.GetLatestCommit(ctx, owner, repository, branch)
	if err != nil {
		return "", err
	}
	commitSHA := commit.Hash
	branch = vcsutils.AddBranchPrefix(branch)
	client.logger.Debug("uploading code scanning for", repository, "/", branch)
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return "", err
	}
	sarifID, resp, err := ghClient.CodeScanning.UploadSarif(ctx, owner, repository, &github.SarifAnalysis{
		CommitSHA:   &commitSHA,
		Ref:         &branch,
		Sarif:       &packagedScan,
		CheckoutURI: nil,
	})
	// According to go-github API - successful response will return 202 status code
	// The body of the response will appear in the error, and the Sarif struct will be empty.
	if err != nil && resp.Response.StatusCode != 202 {
		return "", err
	}
	// We are still using the Sarif struct because we need it for the unit-test of this function
	if sarifID != nil && *sarifID.ID != "" {
		return *sarifID.ID, err
	}
	aerr, ok := err.(*github.AcceptedError)
	var result map[string]string
	if ok {
		err = json.Unmarshal(aerr.Raw, &result)
		if err != nil {
			return "", nil
		}
		return result["id"], nil
	}

	return "", nil
}

// DownloadFileFromRepo on GitHub
func (client *GitHubClient) DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) ([]byte, int, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, 0, err
	}
	body, response, err := ghClient.Repositories.DownloadContents(ctx, owner, repository, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return nil, response.StatusCode, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("expected %d status code while received %d status code", http.StatusOK, response.StatusCode)
	}

	bodyContent, err := io.ReadAll(body)
	if err != nil {
		return nil, response.StatusCode, err
	}
	return bodyContent, response.StatusCode, nil
}

func createGitHubHook(token, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) *github.Hook {
	return &github.Hook{
		Events: getGitHubWebhookEvents(webhookEvents...),
		Config: map[string]interface{}{
			"url":          payloadURL,
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
		case vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected:
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

func mapGitHubCommitToCommitInfo(commit *github.RepositoryCommit) CommitInfo {
	parents := make([]string, len(commit.Parents))
	for i, c := range commit.Parents {
		parents[i] = c.GetSHA()
	}
	details := commit.GetCommit()
	return CommitInfo{
		Hash:          commit.GetSHA(),
		AuthorName:    details.GetAuthor().GetName(),
		CommitterName: details.GetCommitter().GetName(),
		Url:           commit.GetURL(),
		Timestamp:     details.GetCommitter().GetDate().UTC().Unix(),
		Message:       details.GetMessage(),
		ParentHashes:  parents,
	}
}

func mapGitHubCommentToCommentInfoList(commentsList []*github.IssueComment) (res []CommentInfo, err error) {
	for _, comment := range commentsList {
		res = append(res, CommentInfo{
			ID:      *comment.ID,
			Content: *comment.Body,
			Created: *comment.CreatedAt,
		})
	}
	return
}

func mapGitHubPullRequestToPullRequestInfoList(pullRequestList []*github.PullRequest) (res []PullRequestInfo, err error) {
	for _, pullRequest := range pullRequestList {
		res = append(res, PullRequestInfo{
			ID: int64(*pullRequest.Number),
			Source: BranchInfo{
				Name:       *pullRequest.Head.Ref,
				Repository: *pullRequest.Head.Repo.Name,
			},
			Target: BranchInfo{
				Name:       *pullRequest.Base.Ref,
				Repository: *pullRequest.Base.Repo.Name,
			},
		})
	}
	return
}

func packScanningResult(data string) (string, error) {
	compressedScan, err := base64.EncodeGzip([]byte(data), 6)
	if err != nil {
		return "", err
	}

	return compressedScan, err
}
