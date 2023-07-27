package vcsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/datastructures"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/grokify/mogo/encoding/base64"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/oauth2"
)

const (
	GitHubCloudApiEndpoint = "https://api.github.com"
	GitHubCloneUrl         = "https://github.com"
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

// GetCommitStatuses on GitHub
func (client *GitHubClient) GetCommitStatuses(ctx context.Context, owner, repository, ref string) (status []CommitStatusInfo, err error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}
	statuses, _, err := ghClient.Repositories.GetCombinedStatus(ctx, owner, repository, ref, nil)
	if err != nil {
		return nil, err
	}
	results := make([]CommitStatusInfo, 0)
	for _, singleStatus := range statuses.Statuses {
		results = append(results, CommitStatusInfo{
			State:         commitStatusAsStringToStatus(*singleStatus.State),
			Description:   singleStatus.GetDescription(),
			DetailsUrl:    singleStatus.GetTargetURL(),
			Creator:       singleStatus.GetCreator().GetName(),
			LastUpdatedAt: singleStatus.GetUpdatedAt(),
			CreatedAt:     singleStatus.GetCreatedAt(),
		})
	}
	return results, err
}

// DownloadRepository on GitHub
func (client *GitHubClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	client.logger.Debug("Getting GitHub archive link to download")
	baseURL, _, err := ghClient.Repositories.GetArchiveLink(ctx, owner, repository, github.Tarball,
		&github.RepositoryContentGetOptions{Ref: branch}, true)
	if err != nil {
		return err
	}
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
	client.logger.Info(repository, successfulRepoDownload)
	err = vcsutils.Untar(localPath, resp.Body, true)
	if err != nil {
		return err
	}

	client.logger.Info(successfulRepoExtraction)
	return vcsutils.CreateDotGitFolderWithRemote(localPath, vcsutils.RemoteName, getGitHubGitRemoteUrl(client, owner, repository))
}

// CreatePullRequest on GitHub
func (client *GitHubClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch,
	title, description string) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	head := owner + ":" + sourceBranch
	client.logger.Debug(creatingPullRequest, title)
	_, _, err = ghClient.PullRequests.Create(ctx, owner, repository, &github.NewPullRequest{
		Title: &title,
		Body:  &description,
		Head:  &head,
		Base:  &targetBranch,
	})
	return err
}

// UpdatePullRequest on GitHub
func (client *GitHubClient) UpdatePullRequest(ctx context.Context, owner, repository, title, body, targetBranchName string, id int, state vcsutils.PullRequestState) error {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	client.logger.Debug(updatingPullRequest, id)
	var baseRef *github.PullRequestBranch
	if targetBranchName != "" {
		baseRef = &github.PullRequestBranch{Ref: &targetBranchName}
	}
	pullRequest := &github.PullRequest{
		Body:  &body,
		Title: &title,
		State: vcsutils.MapPullRequestState(&state),
		Base:  baseRef,
	}
	_, _, err = ghClient.PullRequests.Edit(ctx, owner, repository, id, pullRequest)
	return err
}

// ListOpenPullRequestsWithBody on GitHub
func (client *GitHubClient) ListOpenPullRequestsWithBody(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	return client.getOpenPullRequests(ctx, owner, repository, true)
}

// ListOpenPullRequests on GitHub
func (client *GitHubClient) ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	return client.getOpenPullRequests(ctx, owner, repository, false)
}

func (client *GitHubClient) getOpenPullRequests(ctx context.Context, owner, repository string, withBody bool) ([]PullRequestInfo, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}
	client.logger.Debug(fetchingOpenPullRequests, repository)
	pullRequests, _, err := ghClient.PullRequests.List(ctx, owner, repository, &github.PullRequestListOptions{
		State: "open",
	})
	if err != nil {
		return []PullRequestInfo{}, err
	}
	return mapGitHubPullRequestToPullRequestInfoList(pullRequests, withBody)
}

func (client *GitHubClient) GetPullRequestByID(ctx context.Context, owner, repository string, pullRequestId int) (PullRequestInfo, error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return PullRequestInfo{}, err
	}
	client.logger.Debug(fetchingPullRequestById, repository)
	pullRequest, response, err := ghClient.PullRequests.Get(ctx, owner, repository, pullRequestId)
	if err != nil || response.StatusCode != http.StatusOK {
		return PullRequestInfo{}, err
	}

	sourceBranch, err1 := extractBranchFromLabel(vcsutils.DefaultIfNotNil(pullRequest.Head.Label))
	targetBranch, err2 := extractBranchFromLabel(vcsutils.DefaultIfNotNil(pullRequest.Base.Label))
	err = errors.Join(err1, err2)
	if err != nil {
		return PullRequestInfo{}, err
	}

	prInfo := PullRequestInfo{
		ID: int64(pullRequestId),
		Source: BranchInfo{
			Name:       sourceBranch,
			Repository: vcsutils.DefaultIfNotNil(pullRequest.Head.Repo.Name),
			Owner:      vcsutils.DefaultIfNotNil(pullRequest.Head.Repo.Owner.Login),
		},
		Target: BranchInfo{
			Name:       targetBranch,
			Repository: vcsutils.DefaultIfNotNil(pullRequest.Base.Repo.Name),
			Owner:      vcsutils.DefaultIfNotNil(pullRequest.Base.Repo.Owner.Login),
		},
	}
	return prInfo, nil
}

// Extracts branch name from the following expected label format repo:branch
func extractBranchFromLabel(label string) (string, error) {
	split := strings.Split(label, ":")
	if len(split) <= 1 {
		return "", fmt.Errorf("bad label format %s", label)
	}
	return split[1], nil
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

// DeletePullRequestComment on GitHub
func (client *GitHubClient) DeletePullRequestComment(ctx context.Context, owner, repository string, _, commentID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return err
	}
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return err
	}
	resp, err := ghClient.Issues.DeleteComment(ctx, owner, repository, int64(commentID))
	if err != nil {
		return err
	}
	var statusCode int
	if resp.Response != nil {
		statusCode = resp.Response.StatusCode
	}
	if statusCode != http.StatusNoContent && statusCode != http.StatusOK {
		return fmt.Errorf("expected %d status code while received %d status code", http.StatusNoContent, resp.Response.StatusCode)
	}
	return nil
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
	return RepositoryInfo{RepositoryVisibility: getGitHubRepositoryVisibility(repo), CloneInfo: CloneInfo{HTTP: repo.GetCloneURL(), SSH: repo.GetSSHURL()}}, nil
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

	label, ghResponse, err := ghClient.Issues.GetLabel(ctx, owner, repository, name)
	if err != nil {
		if ghResponse.Response.StatusCode == http.StatusNotFound {
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
	client.logger.Debug(uploadingCodeScanning, repository, "/", branch)
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
func (client *GitHubClient) DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) (content []byte, statusCode int, err error) {
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, 0, err
	}
	body, ghResponse, err := ghClient.Repositories.DownloadContents(ctx, owner, repository, path, &github.RepositoryContentGetOptions{Ref: branch})
	defer func() {
		if body != nil {
			err = errors.Join(err, body.Close())
		}
	}()
	if ghResponse != nil && ghResponse.Response != nil {
		statusCode = ghResponse.StatusCode
	}
	if err != nil && statusCode != http.StatusOK {
		err = fmt.Errorf("expected %d status code while received %d status code with error:\n%s", http.StatusOK, ghResponse.StatusCode, err)
		return
	}
	if body != nil {
		content, err = io.ReadAll(body)
	}
	return
}

// GetRepositoryEnvironmentInfo on GitHub
func (client *GitHubClient) GetRepositoryEnvironmentInfo(ctx context.Context, owner, repository, name string) (RepositoryEnvironmentInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "name": name})
	if err != nil {
		return RepositoryEnvironmentInfo{}, err
	}
	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return RepositoryEnvironmentInfo{}, err
	}

	environment, resp, err := ghClient.Repositories.GetEnvironment(ctx, owner, repository, name)
	if err != nil {
		return RepositoryEnvironmentInfo{}, err
	}
	if err = vcsutils.CheckResponseStatusWithBody(resp.Response, http.StatusOK); err != nil {
		return RepositoryEnvironmentInfo{}, err
	}

	reviewers, err := extractGitHubEnvironmentReviewers(environment)
	if err != nil {
		return RepositoryEnvironmentInfo{}, err
	}

	return RepositoryEnvironmentInfo{
		Name:      *environment.Name,
		Url:       *environment.URL,
		Reviewers: reviewers,
	}, err
}

func (client *GitHubClient) GetModifiedFiles(ctx context.Context, owner, repository, refBefore, refAfter string) ([]string, error) {
	if err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"refBefore":  refBefore,
		"refAfter":   refAfter,
	}); err != nil {
		return nil, err
	}

	ghClient, err := client.buildGithubClient(ctx)
	if err != nil {
		return nil, err
	}

	// According to the https://docs.github.com/en/rest/commits/commits?apiVersion=2022-11-28#compare-two-commits
	// the list of changed files is always returned with the first page fully,
	// so we don't need to iterate over other pages to get additional info about the files.
	// And we also do not need info about the change that is why we can limit only to a single entity.
	listOptions := &github.ListOptions{PerPage: 1}
	comparison, resp, err := ghClient.Repositories.CompareCommits(ctx, owner, repository, refBefore, refAfter, listOptions)
	if err != nil {
		return nil, err
	}
	if err = vcsutils.CheckResponseStatusWithBody(resp.Response, http.StatusOK); err != nil {
		return nil, err
	}

	fileNamesSet := datastructures.MakeSet[string]()
	for _, file := range comparison.Files {
		fileNamesSet.Add(vcsutils.DefaultIfNotNil(file.Filename))
		fileNamesSet.Add(vcsutils.DefaultIfNotNil(file.PreviousFilename))
	}
	_ = fileNamesSet.Remove("") // Make sure there are no blank filepath.
	fileNamesList := fileNamesSet.ToSlice()
	sort.Strings(fileNamesList)
	return fileNamesList, nil
}

// Extract code reviewers from environment
func extractGitHubEnvironmentReviewers(environment *github.Environment) ([]string, error) {
	var reviewers []string
	protectionRules := environment.ProtectionRules
	if protectionRules == nil {
		return reviewers, nil
	}
	reviewerStruct := repositoryEnvironmentReviewer{}
	for _, rule := range protectionRules {
		for _, reviewer := range rule.Reviewers {
			if err := mapstructure.Decode(reviewer.Reviewer, &reviewerStruct); err != nil {
				return []string{}, err
			}
			reviewers = append(reviewers, reviewerStruct.Login)
		}
	}
	return reviewers, nil
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
	events := datastructures.MakeSet[string]()
	for _, event := range webhookEvents {
		switch event {
		case vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrMerged, vcsutils.PrRejected:
			events.Add("pull_request")
		case vcsutils.Push, vcsutils.TagPushed, vcsutils.TagRemoved:
			events.Add("push")
		}
	}
	return events.ToSlice()
}

func getGitHubRepositoryVisibility(repo *github.Repository) RepositoryVisibility {
	switch *repo.Visibility {
	case "public":
		return Public
	case "internal":
		return Internal
	default:
		return Private
	}
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

func mapGitHubPullRequestToPullRequestInfoList(pullRequestList []*github.PullRequest, withBody bool) (res []PullRequestInfo, err error) {
	for _, pullRequest := range pullRequestList {
		var body string
		if withBody {
			body = *pullRequest.Body
		}
		res = append(res, PullRequestInfo{
			ID:   int64(vcsutils.DefaultIfNotNil(pullRequest.Number)),
			Body: body,
			Source: BranchInfo{
				Name:       vcsutils.DefaultIfNotNil(pullRequest.Head.Ref),
				Repository: vcsutils.DefaultIfNotNil(pullRequest.Head.Repo.Name),
				Owner:      vcsutils.DefaultIfNotNil(pullRequest.Head.Repo.Owner.Login),
			},
			Target: BranchInfo{
				Name:       vcsutils.DefaultIfNotNil(pullRequest.Base.Ref),
				Repository: vcsutils.DefaultIfNotNil(pullRequest.Base.Repo.Name),
				Owner:      vcsutils.DefaultIfNotNil(pullRequest.Base.Repo.Owner.Login),
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

func getGitHubGitRemoteUrl(client *GitHubClient, owner, repo string) string {
	if client.vcsInfo.APIEndpoint == GitHubCloudApiEndpoint {
		return fmt.Sprintf("%s/%s/%s.git", GitHubCloneUrl, owner, repo)
	}
	return fmt.Sprintf("%s/%s/%s.git", client.vcsInfo.APIEndpoint, owner, repo)
}

type repositoryEnvironmentReviewer struct {
	Login string `mapstructure:"login"`
}
