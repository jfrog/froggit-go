package vcsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-github/v56/github"
	"github.com/grokify/mogo/encoding/base64"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetries               = 5
	retriesIntervalMilliSecs = 60000
	// https://github.com/orgs/community/discussions/27190
	githubPrContentSizeLimit = 65536
)

var rateLimitRetryStatuses = []int{http.StatusForbidden, http.StatusTooManyRequests}

type GitHubRateLimitExecutionHandler func() (*github.Response, error)

type GitHubRateLimitRetryExecutor struct {
	vcsutils.RetryExecutor
	GitHubRateLimitExecutionHandler
}

func (ghe *GitHubRateLimitRetryExecutor) Execute() error {
	ghe.ExecutionHandler = func() (bool, error) {
		ghResponse, err := ghe.GitHubRateLimitExecutionHandler()
		return shouldRetryIfRateLimitExceeded(ghResponse, err), err
	}
	return ghe.RetryExecutor.Execute()
}

// GitHubClient API version 3
type GitHubClient struct {
	vcsInfo                VcsInfo
	rateLimitRetryExecutor GitHubRateLimitRetryExecutor
	logger                 vcsutils.Log
	ghClient               *github.Client
}

// NewGitHubClient create a new GitHubClient
func NewGitHubClient(vcsInfo VcsInfo, logger vcsutils.Log) (*GitHubClient, error) {
	ghClient, err := buildGithubClient(vcsInfo, logger)
	if err != nil {
		return nil, err
	}
	return &GitHubClient{
			vcsInfo:  vcsInfo,
			logger:   logger,
			ghClient: ghClient,
			rateLimitRetryExecutor: GitHubRateLimitRetryExecutor{RetryExecutor: vcsutils.RetryExecutor{
				Logger:                   logger,
				MaxRetries:               maxRetries,
				RetriesIntervalMilliSecs: retriesIntervalMilliSecs},
			}},
		nil
}

func (client *GitHubClient) runWithRateLimitRetries(handler func() (*github.Response, error)) error {
	client.rateLimitRetryExecutor.GitHubRateLimitExecutionHandler = handler
	return client.rateLimitRetryExecutor.Execute()
}

// TestConnection on GitHub
func (client *GitHubClient) TestConnection(ctx context.Context) error {
	_, _, err := client.ghClient.Meta.Zen(ctx)
	return err
}

func buildGithubClient(vcsInfo VcsInfo, logger vcsutils.Log) (*github.Client, error) {
	httpClient := &http.Client{}
	if vcsInfo.Token != "" {
		httpClient = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: vcsInfo.Token}))
	}
	ghClient := github.NewClient(httpClient)
	if vcsInfo.APIEndpoint != "" {
		baseURL, err := url.Parse(strings.TrimSuffix(vcsInfo.APIEndpoint, "/") + "/")
		if err != nil {
			return nil, err
		}
		logger.Info("Using API endpoint:", baseURL)
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

	readOnly := permission != ReadWrite
	key := github.Key{
		Key:      &publicKey,
		Title:    &keyName,
		ReadOnly: &readOnly,
	}

	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		_, ghResponse, err := client.ghClient.Repositories.CreateKey(ctx, owner, repository, &key)
		return ghResponse, err
	})
}

// ListRepositories on GitHub
func (client *GitHubClient) ListRepositories(ctx context.Context) (results map[string][]string, err error) {
	results = make(map[string][]string)
	for nextPage := 1; ; nextPage++ {
		var repositoriesInPage []*github.Repository
		var ghResponse *github.Response
		err = client.runWithRateLimitRetries(func() (*github.Response, error) {
			repositoriesInPage, ghResponse, err = client.executeListRepositoriesInPage(ctx, nextPage)
			return ghResponse, err
		})
		if err != nil {
			return
		}

		for _, repo := range repositoriesInPage {
			results[*repo.Owner.Login] = append(results[*repo.Owner.Login], *repo.Name)
		}
		if nextPage+1 > ghResponse.LastPage {
			break
		}
	}
	return
}

func (client *GitHubClient) executeListRepositoriesInPage(ctx context.Context, page int) ([]*github.Repository, *github.Response, error) {
	options := &github.RepositoryListOptions{ListOptions: github.ListOptions{Page: page}}
	return client.ghClient.Repositories.List(ctx, "", options)
}

// ListBranches on GitHub
func (client *GitHubClient) ListBranches(ctx context.Context, owner, repository string) (branchList []string, err error) {
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		branchList, ghResponse, err = client.executeListBranch(ctx, owner, repository)
		return ghResponse, err
	})
	return
}

func (client *GitHubClient) executeListBranch(ctx context.Context, owner, repository string) ([]string, *github.Response, error) {
	branches, ghResponse, err := client.ghClient.Repositories.ListBranches(ctx, owner, repository, nil)
	if err != nil {
		return []string{}, ghResponse, err
	}

	branchList := make([]string, 0, len(branches))
	for _, branch := range branches {
		branchList = append(branchList, *branch.Name)
	}
	return branchList, ghResponse, nil
}

// CreateWebhook on GitHub
func (client *GitHubClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadURL string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	token := vcsutils.CreateToken()
	hook := createGitHubHook(token, payloadURL, webhookEvents...)
	var ghResponseHook *github.Hook
	var err error
	if err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		ghResponseHook, ghResponse, err = client.ghClient.Repositories.CreateHook(ctx, owner, repository, hook)
		return ghResponse, err
	}); err != nil {
		return "", "", err
	}

	return strconv.FormatInt(*ghResponseHook.ID, 10), token, nil
}

// UpdateWebhook on GitHub
func (client *GitHubClient) UpdateWebhook(ctx context.Context, owner, repository, _, payloadURL, token,
	webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	webhookIDInt64, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		return err
	}

	hook := createGitHubHook(token, payloadURL, webhookEvents...)
	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		_, ghResponse, err = client.ghClient.Repositories.EditHook(ctx, owner, repository, webhookIDInt64, hook)
		return ghResponse, err
	})
}

// DeleteWebhook on GitHub
func (client *GitHubClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	webhookIDInt64, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		return err
	}

	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		return client.ghClient.Repositories.DeleteHook(ctx, owner, repository, webhookIDInt64)
	})
}

// SetCommitStatus on GitHub
func (client *GitHubClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref,
	title, description, detailsURL string) error {
	state := getGitHubCommitState(commitStatus)
	status := &github.RepoStatus{
		Context:     &title,
		TargetURL:   &detailsURL,
		State:       &state,
		Description: &description,
	}

	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		_, ghResponse, err := client.ghClient.Repositories.CreateStatus(ctx, owner, repository, ref, status)
		return ghResponse, err
	})
}

// GetCommitStatuses on GitHub
func (client *GitHubClient) GetCommitStatuses(ctx context.Context, owner, repository, ref string) (statusInfoList []CommitStatusInfo, err error) {
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		statusInfoList, ghResponse, err = client.executeGetCommitStatuses(ctx, owner, repository, ref)
		return ghResponse, err
	})
	return
}

func (client *GitHubClient) executeGetCommitStatuses(ctx context.Context, owner, repository, ref string) (statusInfoList []CommitStatusInfo, ghResponse *github.Response, err error) {
	statuses, ghResponse, err := client.ghClient.Repositories.GetCombinedStatus(ctx, owner, repository, ref, nil)
	if err != nil {
		return
	}

	for _, singleStatus := range statuses.Statuses {
		statusInfoList = append(statusInfoList, CommitStatusInfo{
			State:         commitStatusAsStringToStatus(*singleStatus.State),
			Description:   singleStatus.GetDescription(),
			DetailsUrl:    singleStatus.GetTargetURL(),
			Creator:       singleStatus.GetCreator().GetName(),
			LastUpdatedAt: singleStatus.GetUpdatedAt().Time,
			CreatedAt:     singleStatus.GetCreatedAt().Time,
		})
	}
	return
}

// DownloadRepository on GitHub
func (client *GitHubClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) (err error) {
	// Get the archive download link from GitHub
	var baseURL *url.URL
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		baseURL, ghResponse, err = client.executeGetArchiveLink(ctx, owner, repository, branch)
		return ghResponse, err
	})
	if err != nil {
		return
	}

	// Download the archive
	httpResponse, err := executeDownloadArchiveFromLink(baseURL.String())
	if err != nil {
		return
	}
	defer func() { err = errors.Join(err, httpResponse.Body.Close()) }()
	client.logger.Info(repository, vcsutils.SuccessfulRepoDownload)

	// Untar the archive
	if err = vcsutils.Untar(localPath, httpResponse.Body, true); err != nil {
		return
	}
	client.logger.Info(vcsutils.SuccessfulRepoExtraction)

	repositoryInfo, err := client.GetRepositoryInfo(ctx, owner, repository)
	if err != nil {
		return
	}
	// Create a .git folder in the archive with the remote repository HTTP clone url
	err = vcsutils.CreateDotGitFolderWithRemote(localPath, vcsutils.RemoteName, repositoryInfo.CloneInfo.HTTP)
	return
}

func (client *GitHubClient) executeGetArchiveLink(ctx context.Context, owner, repository, branch string) (baseURL *url.URL, ghResponse *github.Response, err error) {
	client.logger.Debug("Getting GitHub archive link to download")
	return client.ghClient.Repositories.GetArchiveLink(ctx, owner, repository, github.Tarball,
		&github.RepositoryContentGetOptions{Ref: branch}, 5)
}

func executeDownloadArchiveFromLink(baseURL string) (*http.Response, error) {
	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, err
	}
	httpResponse, err := httpClient.Do(req)
	if err != nil {
		return httpResponse, err
	}
	return httpResponse, vcsutils.CheckResponseStatusWithBody(httpResponse, http.StatusOK)
}

func (client *GitHubClient) GetPullRequestCommentSizeLimit() int {
	return githubPrContentSizeLimit
}

func (client *GitHubClient) GetPullRequestDetailsSizeLimit() int {
	return githubPrContentSizeLimit
}

// CreatePullRequest on GitHub
func (client *GitHubClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) error {
	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		return client.executeCreatePullRequest(ctx, owner, repository, sourceBranch, targetBranch, title, description)
	})
}

func (client *GitHubClient) executeCreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) (*github.Response, error) {
	head := owner + ":" + sourceBranch
	client.logger.Debug(vcsutils.CreatingPullRequest, title)

	_, ghResponse, err := client.ghClient.PullRequests.Create(ctx, owner, repository, &github.NewPullRequest{
		Title: &title,
		Body:  &description,
		Head:  &head,
		Base:  &targetBranch,
	})
	return ghResponse, err
}

// UpdatePullRequest on GitHub
func (client *GitHubClient) UpdatePullRequest(ctx context.Context, owner, repository, title, body, targetBranchName string, id int, state vcsutils.PullRequestState) error {
	client.logger.Debug(vcsutils.UpdatingPullRequest, id)
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

	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		_, ghResponse, err := client.ghClient.PullRequests.Edit(ctx, owner, repository, id, pullRequest)
		return ghResponse, err
	})
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
	var pullRequests []*github.PullRequest
	client.logger.Debug(vcsutils.FetchingOpenPullRequests, repository)
	err := client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		var err error
		pullRequests, ghResponse, err = client.ghClient.PullRequests.List(ctx, owner, repository, &github.PullRequestListOptions{State: "open"})
		return ghResponse, err
	})
	if err != nil {
		return []PullRequestInfo{}, err
	}

	return mapGitHubPullRequestToPullRequestInfoList(pullRequests, withBody)
}

func (client *GitHubClient) GetPullRequestByID(ctx context.Context, owner, repository string, pullRequestId int) (PullRequestInfo, error) {
	var pullRequest *github.PullRequest
	var ghResponse *github.Response
	var err error
	client.logger.Debug(vcsutils.FetchingPullRequestById, repository)
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		pullRequest, ghResponse, err = client.ghClient.PullRequests.Get(ctx, owner, repository, pullRequestId)
		return ghResponse, err
	})
	if err != nil {
		return PullRequestInfo{}, err
	}

	if err = vcsutils.CheckResponseStatusWithBody(ghResponse.Response, http.StatusOK); err != nil {
		return PullRequestInfo{}, err
	}

	return mapGitHubPullRequestToPullRequestInfo(pullRequest, false)
}

func mapGitHubPullRequestToPullRequestInfo(ghPullRequest *github.PullRequest, withBody bool) (PullRequestInfo, error) {
	var sourceBranch, targetBranch string
	var err1, err2 error
	if ghPullRequest != nil && ghPullRequest.Head != nil && ghPullRequest.Base != nil {
		sourceBranch, err1 = extractBranchFromLabel(vcsutils.DefaultIfNotNil(ghPullRequest.Head.Label))
		targetBranch, err2 = extractBranchFromLabel(vcsutils.DefaultIfNotNil(ghPullRequest.Base.Label))
		err := errors.Join(err1, err2)
		if err != nil {
			return PullRequestInfo{}, err
		}
	}

	var sourceRepoName, sourceRepoOwner string
	if ghPullRequest.Head.Repo == nil {
		return PullRequestInfo{}, errors.New("the source repository information is missing when fetching the pull request details")
	}
	if ghPullRequest.Head.Repo.Owner == nil {
		return PullRequestInfo{}, errors.New("the source repository owner name is missing when fetching the pull request details")
	}
	sourceRepoName = vcsutils.DefaultIfNotNil(ghPullRequest.Head.Repo.Name)
	sourceRepoOwner = vcsutils.DefaultIfNotNil(ghPullRequest.Head.Repo.Owner.Login)

	var targetRepoName, targetRepoOwner string
	if ghPullRequest.Base.Repo == nil {
		return PullRequestInfo{}, errors.New("the target repository information is missing when fetching the pull request details")
	}
	if ghPullRequest.Base.Repo.Owner == nil {
		return PullRequestInfo{}, errors.New("the target repository owner name is missing when fetching the pull request details")
	}
	targetRepoName = vcsutils.DefaultIfNotNil(ghPullRequest.Base.Repo.Name)
	targetRepoOwner = vcsutils.DefaultIfNotNil(ghPullRequest.Base.Repo.Owner.Login)

	var body string
	if withBody {
		body = vcsutils.DefaultIfNotNil(ghPullRequest.Body)
	}

	return PullRequestInfo{
		ID:     int64(vcsutils.DefaultIfNotNil(ghPullRequest.Number)),
		URL:    vcsutils.DefaultIfNotNil(ghPullRequest.HTMLURL),
		Body:   body,
		Author: vcsutils.DefaultIfNotNil(ghPullRequest.User.Login),
		Source: BranchInfo{
			Name:       sourceBranch,
			Repository: sourceRepoName,
			Owner:      sourceRepoOwner,
		},
		Target: BranchInfo{
			Name:       targetBranch,
			Repository: targetRepoName,
			Owner:      targetRepoOwner,
		},
	}, nil
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

	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		// We use the Issues API to add a regular comment. The PullRequests API adds a code review comment.
		_, ghResponse, err = client.ghClient.Issues.CreateComment(ctx, owner, repository, pullRequestID, &github.IssueComment{Body: &content})
		return ghResponse, err
	})
}

// AddPullRequestReviewComments on GitHub
func (client *GitHubClient) AddPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...PullRequestComment) error {
	prID := strconv.Itoa(pullRequestID)
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "pullRequestID": prID})
	if err != nil {
		return err
	}
	if len(comments) == 0 {
		return errors.New(vcsutils.ErrNoCommentsProvided)
	}

	var commits []*github.RepositoryCommit
	var ghResponse *github.Response
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		commits, ghResponse, err = client.ghClient.PullRequests.ListCommits(ctx, owner, repository, pullRequestID, nil)
		return ghResponse, err
	})
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		return errors.New("could not fetch the commits list for pull request " + prID)
	}

	latestCommitSHA := commits[len(commits)-1].GetSHA()

	for _, comment := range comments {
		err = client.runWithRateLimitRetries(func() (*github.Response, error) {
			ghResponse, err = client.executeCreatePullRequestReviewComment(ctx, owner, repository, latestCommitSHA, pullRequestID, comment)
			return ghResponse, err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *GitHubClient) executeCreatePullRequestReviewComment(ctx context.Context, owner, repository, latestCommitSHA string, pullRequestID int, comment PullRequestComment) (*github.Response, error) {
	filePath := filepath.Clean(comment.NewFilePath)
	startLine := &comment.NewStartLine
	// GitHub API won't accept 'start_line' if it equals the end line
	if *startLine == comment.NewEndLine {
		startLine = nil
	}
	_, ghResponse, err := client.ghClient.PullRequests.CreateComment(ctx, owner, repository, pullRequestID, &github.PullRequestComment{
		CommitID:  &latestCommitSHA,
		Body:      &comment.Content,
		StartLine: startLine,
		Line:      &comment.NewEndLine,
		Path:      &filePath,
	})
	if err != nil {
		err = fmt.Errorf("could not create a code review comment for <%s/%s> in pull request %d. error received: %w",
			owner, repository, pullRequestID, err)
	}
	return ghResponse, err
}

// ListPullRequestReviewComments on GitHub
func (client *GitHubClient) ListPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}

	commentsInfoList := []CommentInfo{}
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		commentsInfoList, ghResponse, err = client.executeListPullRequestReviewComments(ctx, owner, repository, pullRequestID)
		return ghResponse, err
	})
	return commentsInfoList, err
}

func (client *GitHubClient) executeListPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, *github.Response, error) {
	commentsList, ghResponse, err := client.ghClient.PullRequests.ListComments(ctx, owner, repository, pullRequestID, nil)
	if err != nil {
		return []CommentInfo{}, ghResponse, err
	}
	commentsInfoList := []CommentInfo{}
	for _, comment := range commentsList {
		commentsInfoList = append(commentsInfoList, CommentInfo{
			ID:      comment.GetID(),
			Content: comment.GetBody(),
			Created: comment.GetCreatedAt().Time,
		})
	}
	return commentsInfoList, ghResponse, nil
}

// ListPullRequestComments on GitHub
func (client *GitHubClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return []CommentInfo{}, err
	}

	var commentsList []*github.IssueComment
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		commentsList, ghResponse, err = client.ghClient.Issues.ListComments(ctx, owner, repository, pullRequestID, &github.IssueListCommentsOptions{})
		return ghResponse, err
	})

	if err != nil {
		return []CommentInfo{}, err
	}

	return mapGitHubIssuesCommentToCommentInfoList(commentsList)
}

// DeletePullRequestReviewComments on GitHub
func (client *GitHubClient) DeletePullRequestReviewComments(ctx context.Context, owner, repository string, _ int, comments ...CommentInfo) error {
	for _, comment := range comments {
		commentID := comment.ID
		err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "commentID": strconv.FormatInt(commentID, 10)})
		if err != nil {
			return err
		}

		err = client.runWithRateLimitRetries(func() (*github.Response, error) {
			return client.executeDeletePullRequestReviewComment(ctx, owner, repository, commentID)
		})
		if err != nil {
			return err
		}

	}
	return nil
}

func (client *GitHubClient) executeDeletePullRequestReviewComment(ctx context.Context, owner, repository string, commentID int64) (*github.Response, error) {
	ghResponse, err := client.ghClient.PullRequests.DeleteComment(ctx, owner, repository, commentID)
	if err != nil {
		err = fmt.Errorf("could not delete pull request review comment: %w", err)
	}
	return ghResponse, err
}

// DeletePullRequestComment on GitHub
func (client *GitHubClient) DeletePullRequestComment(ctx context.Context, owner, repository string, _, commentID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return err
	}
	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		return client.executeDeletePullRequestComment(ctx, owner, repository, commentID)
	})
}

func (client *GitHubClient) executeDeletePullRequestComment(ctx context.Context, owner, repository string, commentID int) (*github.Response, error) {
	ghResponse, err := client.ghClient.Issues.DeleteComment(ctx, owner, repository, int64(commentID))
	if err != nil {
		return ghResponse, err
	}

	var statusCode int
	if ghResponse.Response != nil {
		statusCode = ghResponse.Response.StatusCode
	}
	if statusCode != http.StatusNoContent && statusCode != http.StatusOK {
		return ghResponse, fmt.Errorf("expected %d status code while received %d status code", http.StatusNoContent, ghResponse.Response.StatusCode)
	}

	return ghResponse, nil
}

// GetLatestCommit on GitHub
func (client *GitHubClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	commits, err := client.GetCommits(ctx, owner, repository, branch)
	if err != nil {
		return CommitInfo{}, err
	}
	latestCommit := CommitInfo{}
	if len(commits) > 0 {
		latestCommit = commits[0]
	}
	return latestCommit, nil
}

// GetCommits on GitHub
func (client *GitHubClient) GetCommits(ctx context.Context, owner, repository, branch string) ([]CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"branch":     branch,
	})
	if err != nil {
		return nil, err
	}

	var commitsInfo []CommitInfo
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		listOptions := &github.CommitsListOptions{
			SHA: branch,
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: vcsutils.NumberOfCommitsToFetch,
			},
		}
		commitsInfo, ghResponse, err = client.executeGetCommits(ctx, owner, repository, listOptions)
		return ghResponse, err
	})
	return commitsInfo, err
}

// GetCommitsWithQueryOptions on GitHub
func (client *GitHubClient) GetCommitsWithQueryOptions(ctx context.Context, owner, repository string, listOptions GitCommitsQueryOptions) ([]CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
	})
	if err != nil {
		return nil, err
	}
	var commitsInfo []CommitInfo
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		commitsInfo, ghResponse, err = client.executeGetCommits(ctx, owner, repository, convertToGitHubCommitsListOptions(listOptions))
		return ghResponse, err
	})
	return commitsInfo, err
}

func convertToGitHubCommitsListOptions(listOptions GitCommitsQueryOptions) *github.CommitsListOptions {
	return &github.CommitsListOptions{
		Since: listOptions.Since,
		Until: time.Now(),
		ListOptions: github.ListOptions{
			Page:    listOptions.Page,
			PerPage: listOptions.PerPage,
		},
	}
}

func (client *GitHubClient) executeGetCommits(ctx context.Context, owner, repository string, listOptions *github.CommitsListOptions) ([]CommitInfo, *github.Response, error) {
	commits, ghResponse, err := client.ghClient.Repositories.ListCommits(ctx, owner, repository, listOptions)
	if err != nil {
		return nil, ghResponse, err
	}

	var commitsInfo []CommitInfo
	for _, commit := range commits {
		commitInfo := mapGitHubCommitToCommitInfo(commit)
		commitsInfo = append(commitsInfo, commitInfo)
	}
	return commitsInfo, ghResponse, nil
}

// GetRepositoryInfo on GitHub
func (client *GitHubClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return RepositoryInfo{}, err
	}

	var repo *github.Repository
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		repo, ghResponse, err = client.ghClient.Repositories.Get(ctx, owner, repository)
		return ghResponse, err
	})
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

	var commit *github.RepositoryCommit
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		commit, ghResponse, err = client.ghClient.Repositories.GetCommit(ctx, owner, repository, sha, nil)
		return ghResponse, err
	})
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

	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		_, ghResponse, err = client.ghClient.Issues.CreateLabel(ctx, owner, repository, &github.Label{
			Name:        &labelInfo.Name,
			Description: &labelInfo.Description,
			Color:       &labelInfo.Color,
		})
		return ghResponse, err
	})
}

// GetLabel on GitHub
func (client *GitHubClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "name": name})
	if err != nil {
		return nil, err
	}

	var labelInfo *LabelInfo
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		labelInfo, ghResponse, err = client.executeGetLabel(ctx, owner, repository, name)
		return ghResponse, err
	})
	return labelInfo, err
}

func (client *GitHubClient) executeGetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, *github.Response, error) {
	label, ghResponse, err := client.ghClient.Issues.GetLabel(ctx, owner, repository, name)
	if err != nil {
		if ghResponse != nil && ghResponse.Response != nil && ghResponse.Response.StatusCode == http.StatusNotFound {
			return nil, ghResponse, nil
		}
		return nil, ghResponse, err
	}

	labelInfo := &LabelInfo{
		Name:        *label.Name,
		Description: *label.Description,
		Color:       *label.Color,
	}
	return labelInfo, ghResponse, nil
}

// ListPullRequestLabels on GitHub
func (client *GitHubClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}

	results := []string{}
	for nextPage := 0; ; nextPage++ {
		options := &github.ListOptions{Page: nextPage}
		var labels []*github.Label
		var ghResponse *github.Response
		err = client.runWithRateLimitRetries(func() (*github.Response, error) {
			labels, ghResponse, err = client.ghClient.Issues.ListLabelsByIssue(ctx, owner, repository, pullRequestID, options)
			return ghResponse, err
		})
		if err != nil {
			return nil, err
		}
		for _, label := range labels {
			results = append(results, *label.Name)
		}
		if nextPage+1 >= ghResponse.LastPage {
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

	return client.runWithRateLimitRetries(func() (*github.Response, error) {
		return client.ghClient.Issues.RemoveLabelForIssue(ctx, owner, repository, pullRequestID, name)
	})
}

// UploadCodeScanning to GitHub Security tab
func (client *GitHubClient) UploadCodeScanning(ctx context.Context, owner, repository, branch, sarifContent string) (id string, err error) {
	commit, err := client.GetLatestCommit(ctx, owner, repository, branch)
	if err != nil {
		return
	}

	commitSHA := commit.Hash
	branch = vcsutils.AddBranchPrefix(branch)
	client.logger.Debug(vcsutils.UploadingCodeScanning, repository, "/", branch)

	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		id, ghResponse, err = client.executeUploadCodeScanning(ctx, owner, repository, branch, commitSHA, sarifContent)
		return ghResponse, err
	})
	return
}

func (client *GitHubClient) executeUploadCodeScanning(ctx context.Context, owner, repository, branch, commitSHA, sarifContent string) (id string, ghResponse *github.Response, err error) {
	encodedSarif, err := encodeScanningResult(sarifContent)
	if err != nil {
		return
	}

	sarifID, ghResponse, err := client.ghClient.CodeScanning.UploadSarif(ctx, owner, repository, &github.SarifAnalysis{
		CommitSHA: &commitSHA,
		Ref:       &branch,
		Sarif:     &encodedSarif,
	})

	// According to go-github API - successful ghResponse will return 202 status code
	// The body of the ghResponse will appear in the error, and the Sarif struct will be empty.
	if err != nil && ghResponse.Response.StatusCode != http.StatusAccepted {
		return
	}

	id, err = handleGitHubUploadSarifID(sarifID, err)
	return
}

func handleGitHubUploadSarifID(sarifID *github.SarifID, uploadSarifErr error) (id string, err error) {
	if sarifID != nil && *sarifID.ID != "" {
		id = *sarifID.ID
		return
	}
	var result map[string]string
	var ghAcceptedError *github.AcceptedError
	if errors.As(uploadSarifErr, &ghAcceptedError) {
		if err = json.Unmarshal(ghAcceptedError.Raw, &result); err != nil {
			return
		}
		id = result["id"]
	}
	return
}

// DownloadFileFromRepo on GitHub
func (client *GitHubClient) DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) (content []byte, statusCode int, err error) {
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		content, statusCode, ghResponse, err = client.executeDownloadFileFromRepo(ctx, owner, repository, branch, path)
		return ghResponse, err
	})
	return
}

func (client *GitHubClient) executeDownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) (content []byte, statusCode int, ghResponse *github.Response, err error) {
	body, ghResponse, err := client.ghClient.Repositories.DownloadContents(ctx, owner, repository, path, &github.RepositoryContentGetOptions{Ref: branch})
	defer func() {
		if body != nil {
			err = errors.Join(err, body.Close())
		}
	}()

	if ghResponse == nil || ghResponse.Response == nil {
		return
	}

	statusCode = ghResponse.StatusCode
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

	var repositoryEnvInfo *RepositoryEnvironmentInfo
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		repositoryEnvInfo, ghResponse, err = client.executeGetRepositoryEnvironmentInfo(ctx, owner, repository, name)
		return ghResponse, err
	})
	return *repositoryEnvInfo, err
}

func (client *GitHubClient) executeGetRepositoryEnvironmentInfo(ctx context.Context, owner, repository, name string) (*RepositoryEnvironmentInfo, *github.Response, error) {
	environment, ghResponse, err := client.ghClient.Repositories.GetEnvironment(ctx, owner, repository, name)
	if err != nil {
		return &RepositoryEnvironmentInfo{}, ghResponse, err
	}

	if err = vcsutils.CheckResponseStatusWithBody(ghResponse.Response, http.StatusOK); err != nil {
		return &RepositoryEnvironmentInfo{}, ghResponse, err
	}

	reviewers, err := extractGitHubEnvironmentReviewers(environment)
	if err != nil {
		return &RepositoryEnvironmentInfo{}, ghResponse, err
	}

	return &RepositoryEnvironmentInfo{
			Name:      environment.GetName(),
			Url:       environment.GetURL(),
			Reviewers: reviewers,
		},
		ghResponse,
		nil
}

func (client *GitHubClient) GetModifiedFiles(ctx context.Context, owner, repository, refBefore, refAfter string) ([]string, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"refBefore":  refBefore,
		"refAfter":   refAfter,
	})
	if err != nil {
		return nil, err
	}

	var fileNamesList []string
	err = client.runWithRateLimitRetries(func() (*github.Response, error) {
		var ghResponse *github.Response
		fileNamesList, ghResponse, err = client.executeGetModifiedFiles(ctx, owner, repository, refBefore, refAfter)
		return ghResponse, err
	})
	return fileNamesList, err
}

func (client *GitHubClient) executeGetModifiedFiles(ctx context.Context, owner, repository, refBefore, refAfter string) ([]string, *github.Response, error) {
	// According to the https://docs.github.com/en/rest/commits/commits?apiVersion=2022-11-28#compare-two-commits
	// the list of changed files is always returned with the first page fully,
	// so we don't need to iterate over other pages to get additional info about the files.
	// And we also do not need info about the change that is why we can limit only to a single entity.
	listOptions := &github.ListOptions{PerPage: 1}

	comparison, ghResponse, err := client.ghClient.Repositories.CompareCommits(ctx, owner, repository, refBefore, refAfter, listOptions)
	if err != nil {
		return nil, ghResponse, err
	}

	if err = vcsutils.CheckResponseStatusWithBody(ghResponse.Response, http.StatusOK); err != nil {
		return nil, ghResponse, err
	}

	fileNamesSet := datastructures.MakeSet[string]()
	for _, file := range comparison.Files {
		fileNamesSet.Add(vcsutils.DefaultIfNotNil(file.Filename))
		fileNamesSet.Add(vcsutils.DefaultIfNotNil(file.PreviousFilename))
	}

	_ = fileNamesSet.Remove("") // Make sure there are no blank filepath.
	fileNamesList := fileNamesSet.ToSlice()
	sort.Strings(fileNamesList)

	return fileNamesList, ghResponse, nil
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
		AuthorEmail:   details.GetAuthor().GetEmail(),
	}
}

func mapGitHubIssuesCommentToCommentInfoList(commentsList []*github.IssueComment) (res []CommentInfo, err error) {
	for _, comment := range commentsList {
		res = append(res, CommentInfo{
			ID:      comment.GetID(),
			Content: comment.GetBody(),
			Created: comment.GetCreatedAt().Time,
		})
	}
	return
}

func mapGitHubPullRequestToPullRequestInfoList(pullRequestList []*github.PullRequest, withBody bool) (res []PullRequestInfo, err error) {
	var mappedPullRequest PullRequestInfo
	for _, pullRequest := range pullRequestList {
		mappedPullRequest, err = mapGitHubPullRequestToPullRequestInfo(pullRequest, withBody)
		if err != nil {
			return
		}
		res = append(res, mappedPullRequest)
	}
	return
}

func encodeScanningResult(data string) (string, error) {
	compressedScan, err := base64.EncodeGzip([]byte(data), 6)
	if err != nil {
		return "", err
	}

	return compressedScan, err
}

type repositoryEnvironmentReviewer struct {
	Login string `mapstructure:"login"`
}

func shouldRetryIfRateLimitExceeded(ghResponse *github.Response, requestError error) bool {
	if ghResponse == nil || ghResponse.Response == nil {
		return false
	}

	if !slices.Contains(rateLimitRetryStatuses, ghResponse.StatusCode) {
		return false
	}

	// In case of encountering a rate limit abuse, it's advisable to observe a considerate delay before attempting a retry.
	// This prevents immediate retries within the current sequence, allowing a respectful interval before reattempting the request.
	if requestError != nil && isRateLimitAbuseError(requestError) {
		return false
	}

	body, err := io.ReadAll(ghResponse.Body)
	if err != nil {
		return false
	}
	return strings.Contains(string(body), "rate limit")
}

func isRateLimitAbuseError(requestError error) bool {
	var abuseRateLimitError *github.AbuseRateLimitError
	var rateLimitError *github.RateLimitError
	return errors.As(requestError, &abuseRateLimitError) || errors.As(requestError, &rateLimitError)
}
