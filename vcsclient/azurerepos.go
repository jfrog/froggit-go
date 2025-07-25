package vcsclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
)

const (
	notSupportedOnAzure              = "currently not supported on Azure"
	defaultAzureBaseUrl              = "https://dev.azure.com/"
	azurePullRequestDetailsSizeLimit = 4000
	azurePullRequestCommentSizeLimit = 150000
)

var errAzureGetCommitsWithOptionsNotSupported = fmt.Errorf("get commits with options is %s", notSupportedOnAzure)

// Azure Devops API version 6
type AzureReposClient struct {
	vcsInfo           VcsInfo
	connectionDetails *azuredevops.Connection
	logger            vcsutils.Log
}

// NewAzureReposClient create a new AzureReposClient
func NewAzureReposClient(vcsInfo VcsInfo, logger vcsutils.Log) (*AzureReposClient, error) {
	client := &AzureReposClient{vcsInfo: vcsInfo, logger: logger}
	baseUrl := strings.TrimSuffix(client.vcsInfo.APIEndpoint, "/")
	client.connectionDetails = azuredevops.NewPatConnection(baseUrl, client.vcsInfo.Token)
	return client, nil
}

func (client *AzureReposClient) buildAzureReposClient(ctx context.Context) (git.Client, error) {
	if client.connectionDetails == nil {
		return nil, errors.New("connection details wasn't initialized")
	}
	return git.NewClient(ctx, client.connectionDetails)
}

// TestConnection on Azure Repos
func (client *AzureReposClient) TestConnection(ctx context.Context) error {
	buildClient := azuredevops.NewClient(client.connectionDetails, client.connectionDetails.BaseUrl)
	_, err := buildClient.GetResourceAreas(ctx)
	return err
}

// ListRepositories on Azure Repos
func (client *AzureReposClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}
	repositories := make(map[string][]string)
	resp, err := azureReposGitClient.GetRepositories(ctx, git.GetRepositoriesArgs{Project: &client.vcsInfo.Project})
	if err != nil {
		return repositories, err
	}
	for _, repo := range *resp {
		repositories[client.vcsInfo.Project] = append(repositories[client.vcsInfo.Project], *repo.Name)
	}
	return repositories, nil
}

// ListAppRepositories returns an error since this is not supported in Azure Repos
func (client *AzureReposClient) ListAppRepositories(ctx context.Context) ([]AppRepositoryInfo, error) {
	return nil, getUnsupportedInAzureError("list app repositories")
}

// ListBranches on Azure Repos
func (client *AzureReposClient) ListBranches(ctx context.Context, _, repository string) ([]string, error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}
	var branches []string
	gitBranchStats, err := azureReposGitClient.GetBranches(ctx, git.GetBranchesArgs{Project: &client.vcsInfo.Project, RepositoryId: &repository})
	if err != nil {
		return nil, err
	}
	for _, branch := range *gitBranchStats {
		branches = append(branches, *branch.Name)
	}
	return branches, nil
}

// DownloadRepository on Azure Repos
func (client *AzureReposClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) (err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	// Changing dir to localPath will download the repository there.
	if err = os.Chdir(localPath); err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, os.Chdir(wd))
	}()
	res, err := client.sendDownloadRepoRequest(ctx, repository, branch)
	defer func() {
		if res.Body != nil {
			err = errors.Join(err, res.Body.Close())
		}
	}()
	if err != nil {
		return
	}
	zipFileContent, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}
	err = vcsutils.Unzip(zipFileContent, localPath)
	if err != nil {
		return err
	}
	client.logger.Info(vcsutils.SuccessfulRepoExtraction)
	repoInfo, err := client.GetRepositoryInfo(ctx, owner, repository)
	if err != nil {
		return err
	}
	httpsCloneUrl := repoInfo.CloneInfo.HTTP
	// Generate .git folder with remote details
	return vcsutils.CreateDotGitFolderWithRemote(
		localPath,
		vcsutils.RemoteName,
		httpsCloneUrl)
}

func (client *AzureReposClient) sendDownloadRepoRequest(ctx context.Context, repository string, branch string) (res *http.Response, err error) {
	downloadRepoUrl := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items/items?path=/&versionDescriptor[version]=%s&$format=zip",
		client.connectionDetails.BaseUrl,
		client.vcsInfo.Project,
		repository,
		url.QueryEscape(branch))
	client.logger.Debug("Download url:", downloadRepoUrl)
	headers := map[string]string{
		"Authorization":  client.connectionDetails.AuthorizationString,
		"download":       "true",
		"resolveLfs":     "true",
		"includeContent": "true",
	}
	httpClient := &http.Client{}
	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, downloadRepoUrl, nil); err != nil {
		return
	}
	for key, val := range headers {
		req.Header.Add(key, val)
	}
	if res, err = httpClient.Do(req); err != nil {
		return
	}
	if err = vcsutils.CheckResponseStatusWithBody(res, http.StatusOK); err != nil {
		return &http.Response{}, err
	}
	client.logger.Info(repository, vcsutils.SuccessfulRepoDownload)
	return
}

func (client *AzureReposClient) GetPullRequestCommentSizeLimit() int {
	return azurePullRequestCommentSizeLimit
}

func (client *AzureReposClient) GetPullRequestDetailsSizeLimit() int {
	return azurePullRequestDetailsSizeLimit
}

// CreatePullRequest on Azure Repos
func (client *AzureReposClient) CreatePullRequest(ctx context.Context, _, repository, sourceBranch, targetBranch, title, description string) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	sourceBranch = vcsutils.AddBranchPrefix(sourceBranch)
	targetBranch = vcsutils.AddBranchPrefix(targetBranch)
	client.logger.Debug(vcsutils.CreatingPullRequest, title)
	_, err = azureReposGitClient.CreatePullRequest(ctx, git.CreatePullRequestArgs{
		GitPullRequestToCreate: &git.GitPullRequest{
			Description:   &description,
			SourceRefName: &sourceBranch,
			TargetRefName: &targetBranch,
			Title:         &title,
		},
		RepositoryId: &repository,
		Project:      &client.vcsInfo.Project,
	})
	return err
}

// UpdatePullRequest on Azure Repos
func (client *AzureReposClient) UpdatePullRequest(ctx context.Context, _, repository, title, body, targetBranchName string, prId int, state vcsutils.PullRequestState) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	targetBranchName = vcsutils.AddBranchPrefix(targetBranchName)
	client.logger.Debug(vcsutils.UpdatingPullRequest, prId)
	_, err = azureReposGitClient.UpdatePullRequest(ctx, git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: &git.GitPullRequest{
			Description:   vcsutils.GetNilIfZeroVal(body),
			Status:        azureMapPullRequestState(state),
			TargetRefName: vcsutils.GetNilIfZeroVal(targetBranchName),
			Title:         vcsutils.GetNilIfZeroVal(title),
		},
		RepositoryId:  vcsutils.GetNilIfZeroVal(repository),
		PullRequestId: vcsutils.GetNilIfZeroVal(prId),
		Project:       vcsutils.GetNilIfZeroVal(client.vcsInfo.Project),
	})
	return err
}

// AddPullRequestComment on Azure Repos
func (client *AzureReposClient) AddPullRequestComment(ctx context.Context, _, repository, content string, pullRequestID int) error {
	return client.addPullRequestComment(ctx, repository, pullRequestID, PullRequestComment{CommentInfo: CommentInfo{Content: content}})
}

// AddPullRequestReviewComments on Azure Repos
func (client *AzureReposClient) AddPullRequestReviewComments(ctx context.Context, _, repository string, pullRequestID int, comments ...PullRequestComment) error {
	if len(comments) == 0 {
		return errors.New(vcsutils.ErrNoCommentsProvided)
	}
	for _, comment := range comments {
		if err := client.addPullRequestComment(ctx, repository, pullRequestID, comment); err != nil {
			return err
		}
	}
	return nil
}

func (client *AzureReposClient) addPullRequestComment(ctx context.Context, repository string, pullRequestID int, comment PullRequestComment) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	threadArgs := getThreadArgs(repository, client.vcsInfo.Project, pullRequestID, comment)
	_, err = azureReposGitClient.CreateThread(ctx, threadArgs)
	return err
}

func getThreadArgs(repository, project string, prId int, comment PullRequestComment) git.CreateThreadArgs {
	filePath := vcsutils.GetPullRequestFilePath(comment.NewFilePath)
	return git.CreateThreadArgs{
		CommentThread: &git.GitPullRequestCommentThread{
			Comments: &[]git.Comment{{Content: &comment.Content}},
			Status:   &git.CommentThreadStatusValues.Active,
			ThreadContext: &git.CommentThreadContext{
				FilePath:       &filePath,
				LeftFileStart:  &git.CommentPosition{Line: &comment.OriginalStartLine, Offset: &comment.OriginalStartColumn},
				LeftFileEnd:    &git.CommentPosition{Line: &comment.OriginalEndLine, Offset: &comment.OriginalEndColumn},
				RightFileStart: &git.CommentPosition{Line: &comment.NewStartLine, Offset: &comment.NewStartColumn},
				RightFileEnd:   &git.CommentPosition{Line: &comment.NewEndLine, Offset: &comment.NewEndColumn},
			},
		},
		RepositoryId:  &repository,
		PullRequestId: &prId,
		Project:       &project,
	}
}

func (client *AzureReposClient) ListPullRequestReviews(ctx context.Context, owner, repository string, pullRequestID int) ([]PullRequestReviewDetails, error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}

	reviewers, err := azureReposGitClient.GetPullRequestReviewers(ctx, git.GetPullRequestReviewersArgs{
		RepositoryId:  &repository,
		PullRequestId: &pullRequestID,
		Project:       &client.vcsInfo.Project,
	})
	if err != nil {
		return nil, err
	}

	var reviews []PullRequestReviewDetails
	for _, reviewer := range *reviewers {
		id, err := strconv.ParseInt(*reviewer.Id, 10, 64)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, PullRequestReviewDetails{
			ID:       id,
			Reviewer: *reviewer.DisplayName,
			State:    mapVoteToState(*reviewer.Vote),
		})
	}

	return reviews, nil
}

func (client *AzureReposClient) ListPullRequestsAssociatedWithCommit(ctx context.Context, owner, repository string, commitSHA string) ([]PullRequestInfo, error) {
	return nil, getUnsupportedInAzureError("list pull requests associated with commit")
}

// ListPullRequestReviewComments on Azure Repos
func (client *AzureReposClient) ListPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	return client.ListPullRequestComments(ctx, owner, repository, pullRequestID)
}

// ListPullRequestComments on Azure Repos
func (client *AzureReposClient) ListPullRequestComments(ctx context.Context, _, repository string, pullRequestID int) ([]CommentInfo, error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}
	threads, err := azureReposGitClient.GetThreads(ctx, git.GetThreadsArgs{
		RepositoryId:  &repository,
		PullRequestId: &pullRequestID,
		Project:       &client.vcsInfo.Project,
	})
	if err != nil {
		return nil, err
	}
	var commentInfo []CommentInfo
	for _, thread := range *threads {
		if thread.IsDeleted != nil && *thread.IsDeleted {
			continue
		}
		var commentsAggregator strings.Builder
		for _, comment := range *thread.Comments {
			if comment.IsDeleted != nil && *comment.IsDeleted {
				continue
			}
			_, err = commentsAggregator.WriteString(
				fmt.Sprintf("Author: %s, Id: %d, Content:%s\n",
					*comment.Author.DisplayName,
					*comment.Id,
					*comment.Content))
			if err != nil {
				return nil, err
			}
		}
		commentInfo = append(commentInfo, CommentInfo{
			ID:      int64(*thread.Id),
			Created: thread.PublishedDate.Time,
			Content: commentsAggregator.String(),
		})
	}
	return commentInfo, nil
}

// DeletePullRequestReviewComments on Azure Repos
func (client *AzureReposClient) DeletePullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...CommentInfo) error {
	for _, comment := range comments {
		if err := client.DeletePullRequestComment(ctx, owner, repository, pullRequestID, int(comment.ID)); err != nil {
			return err
		}
	}
	return nil
}

// DeletePullRequestComment on Azure Repos
func (client *AzureReposClient) DeletePullRequestComment(ctx context.Context, _, repository string, pullRequestID, commentID int) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	firstCommentInThreadID := 1
	return azureReposGitClient.DeleteComment(ctx, git.DeleteCommentArgs{
		RepositoryId:  &repository,
		PullRequestId: &pullRequestID,
		ThreadId:      &commentID,
		Project:       &client.vcsInfo.Project,
		CommentId:     &firstCommentInThreadID,
	})
}

// ListOpenPullRequestsWithBody on Azure Repos
func (client *AzureReposClient) ListOpenPullRequestsWithBody(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	return client.getOpenPullRequests(ctx, owner, repository, true)
}

// ListOpenPullRequests on Azure Repos
func (client *AzureReposClient) ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	return client.getOpenPullRequests(ctx, owner, repository, false)
}

func (client *AzureReposClient) getOpenPullRequests(ctx context.Context, owner, repository string, withBody bool) ([]PullRequestInfo, error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}
	client.logger.Debug(vcsutils.FetchingOpenPullRequests, repository)
	pullRequests, err := azureReposGitClient.GetPullRequests(ctx, git.GetPullRequestsArgs{
		RepositoryId:   &repository,
		Project:        &client.vcsInfo.Project,
		SearchCriteria: &git.GitPullRequestSearchCriteria{Status: &git.PullRequestStatusValues.Active},
	})
	if err != nil {
		return nil, err
	}
	var pullRequestsInfo []PullRequestInfo
	for _, pullRequest := range *pullRequests {
		pullRequestDetails := parsePullRequestDetails(client, pullRequest, owner, repository, withBody)
		pullRequestsInfo = append(pullRequestsInfo, pullRequestDetails)
	}
	return pullRequestsInfo, nil
}

// GetPullRequestById in Azure Repos
func (client *AzureReposClient) GetPullRequestByID(ctx context.Context, owner, repository string, pullRequestId int) (pullRequestInfo PullRequestInfo, err error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return
	}
	client.logger.Debug(vcsutils.FetchingPullRequestById, repository)
	pullRequest, err := azureReposGitClient.GetPullRequestById(ctx, git.GetPullRequestByIdArgs{
		PullRequestId: &pullRequestId,
		Project:       &client.vcsInfo.Project,
	})
	if err != nil {
		return
	}
	pullRequestInfo = parsePullRequestDetails(client, *pullRequest, owner, repository, false)
	return
}

// GetLatestCommit on Azure Repos
func (client *AzureReposClient) GetLatestCommit(ctx context.Context, _, repository, branch string) (CommitInfo, error) {
	commitsInfo, err := client.GetCommits(ctx, "", repository, branch)
	if err != nil {
		return CommitInfo{}, err
	}

	var latestCommit CommitInfo
	if len(commitsInfo) > 0 {
		latestCommit = commitsInfo[0]
	}
	return latestCommit, nil
}

// GetCommits on Azure Repos
func (client *AzureReposClient) GetCommits(ctx context.Context, _, repository, branch string) ([]CommitInfo, error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}
	commits, err := azureReposGitClient.GetCommits(ctx, git.GetCommitsArgs{
		RepositoryId:   &repository,
		Project:        &client.vcsInfo.Project,
		SearchCriteria: &git.GitQueryCommitsCriteria{ItemVersion: &git.GitVersionDescriptor{Version: &branch, VersionType: &git.GitVersionTypeValues.Branch}},
	})
	if err != nil {
		return nil, err
	}
	if commits == nil {
		return nil, fmt.Errorf("could not retrieve commits for <%s/%s>", repository, branch)
	}

	var commitsInfo []CommitInfo
	for _, commit := range *commits {
		commitInfo := mapAzureReposCommitsToCommitInfo(commit)
		commitsInfo = append(commitsInfo, commitInfo)
	}
	return commitsInfo, nil
}

func (client *AzureReposClient) GetCommitsWithQueryOptions(ctx context.Context, _, repository string, listOptions GitCommitsQueryOptions) ([]CommitInfo, error) {
	return nil, errAzureGetCommitsWithOptionsNotSupported
}

func mapAzureReposCommitsToCommitInfo(commit git.GitCommitRef) CommitInfo {
	var authorName, authorEmail string
	if commit.Author != nil {
		authorName = vcsutils.DefaultIfNotNil(commit.Author.Name)
		authorEmail = vcsutils.DefaultIfNotNil(commit.Author.Email)
	}
	var committerName string
	var timestamp int64
	if commit.Committer != nil {
		committerName = vcsutils.DefaultIfNotNil(commit.Committer.Name)
		timestamp = vcsutils.DefaultIfNotNil(commit.Committer.Date).Time.Unix()
	}
	return CommitInfo{
		Hash:          vcsutils.DefaultIfNotNil(commit.CommitId),
		AuthorName:    authorName,
		CommitterName: committerName,
		Url:           vcsutils.DefaultIfNotNil(commit.Url),
		Timestamp:     timestamp,
		Message:       vcsutils.DefaultIfNotNil(commit.Comment),
		ParentHashes:  vcsutils.DefaultIfNotNil(commit.Parents),
		AuthorEmail:   authorEmail,
	}
}

func getUnsupportedInAzureError(functionName string) error {
	return fmt.Errorf("%s is currently not supported for Azure Repos", functionName)
}

// AddSshKeyToRepository on Azure Repos
func (client *AzureReposClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission Permission) error {
	return getUnsupportedInAzureError("add ssh key to repository")
}

// GetRepositoryInfo on Azure Repos
func (client *AzureReposClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return RepositoryInfo{}, err
	}
	response, err := azureReposGitClient.GetRepository(ctx, git.GetRepositoryArgs{
		RepositoryId: &repository,
		Project:      &client.vcsInfo.Project,
	})
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("an error occured while retrieving <%s/%s/%s> repository info:\n%s", owner, client.vcsInfo.Project, repository, err.Error())
	}
	if response == nil {
		return RepositoryInfo{}, fmt.Errorf("failed to retreive <%s/%s/%s> repository info, received empty response", owner, client.vcsInfo.Project, repository)
	}
	if response.Project == nil {
		return RepositoryInfo{}, fmt.Errorf("failed to retreive <%s/%s/%s> repository info, received empty project info", owner, client.vcsInfo.Project, repository)
	}

	visibility := Private
	visibilityFromResponse := *response.Project.Visibility
	if visibilityFromResponse == core.ProjectVisibilityValues.Public {
		visibility = Public
	}
	return RepositoryInfo{
		CloneInfo:            CloneInfo{HTTP: *response.RemoteUrl, SSH: *response.SshUrl},
		RepositoryVisibility: visibility,
	}, nil
}

// GetCommitBySha on Azure Repos
func (client *AzureReposClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	return CommitInfo{}, getUnsupportedInAzureError("get commit by sha")
}

// CreateLabel on Azure Repos
func (client *AzureReposClient) CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error {
	return getUnsupportedInAzureError("create label")
}

// GetLabel on Azure Repos
func (client *AzureReposClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	return nil, getUnsupportedInAzureError("get label")
}

// ListPullRequestLabels on Azure Repos
func (client *AzureReposClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	return nil, getUnsupportedInAzureError("list pull request labels")
}

// UnlabelPullRequest on Azure Repos
func (client *AzureReposClient) UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error {
	return getUnsupportedInAzureError("unlabel pull request")
}

// UploadCodeScanning on Azure Repos
func (client *AzureReposClient) UploadCodeScanning(ctx context.Context, owner, repository, branch, scanResults string) (string, error) {
	return "", getUnsupportedInAzureError("upload code scanning")
}

// CreateWebhook on Azure Repos
func (client *AzureReposClient) CreateWebhook(ctx context.Context, owner, repository, branch, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	return "", "", getUnsupportedInAzureError("create webhook")
}

// UpdateWebhook on Azure Repos
func (client *AzureReposClient) UpdateWebhook(ctx context.Context, owner, repository, branch, payloadURL, token, webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	return getUnsupportedInAzureError("update webhook")
}

// DeleteWebhook on Azure Repos
func (client *AzureReposClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	return getUnsupportedInAzureError("delete webhook")
}

// SetCommitStatus on Azure Repos
func (client *AzureReposClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref, title, description, detailsURL string) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	statusState := git.GitStatusState(mapStatusToString(commitStatus))
	commitStatusArgs := git.CreateCommitStatusArgs{
		GitCommitStatusToCreate: &git.GitStatus{
			Description: &description,
			State:       &statusState,
			TargetUrl:   &detailsURL,
			Context: &git.GitStatusContext{
				Name:  &owner,
				Genre: &title,
			},
		},
		CommitId:     &ref,
		RepositoryId: &repository,
		Project:      &client.vcsInfo.Project,
	}
	_, err = azureReposGitClient.CreateCommitStatus(ctx, commitStatusArgs)
	return err
}

// GetCommitStatuses on Azure Repos
func (client *AzureReposClient) GetCommitStatuses(ctx context.Context, owner, repository, ref string) (status []CommitStatusInfo, err error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}
	commitStatusArgs := git.GetStatusesArgs{
		CommitId:     &ref,
		RepositoryId: &repository,
		Project:      &client.vcsInfo.Project,
	}
	resGitStatus, err := azureReposGitClient.GetStatuses(ctx, commitStatusArgs)
	if err != nil {
		return nil, err
	}
	results := make([]CommitStatusInfo, 0)
	for _, singleStatus := range *resGitStatus {
		results = append(results, CommitStatusInfo{
			State:         commitStatusAsStringToStatus(string(*singleStatus.State)),
			Description:   *singleStatus.Description,
			DetailsUrl:    *singleStatus.TargetUrl,
			Creator:       *singleStatus.CreatedBy.DisplayName,
			LastUpdatedAt: extractTimeFromAzuredevopsTime(singleStatus.UpdatedDate),
			CreatedAt:     extractTimeFromAzuredevopsTime(singleStatus.CreationDate),
		})
	}
	return results, err
}

// DownloadFileFromRepo on Azure Repos
func (client *AzureReposClient) DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) ([]byte, int, error) {
	if err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"path":       path,
	}); err != nil {
		return nil, 0, err
	}

	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, 0, err
	}

	trueVal := true
	output, err := azureReposGitClient.GetItemContent(ctx, git.GetItemContentArgs{
		RepositoryId:      &repository,
		Path:              &path,
		Project:           &client.vcsInfo.Project,
		VersionDescriptor: &git.GitVersionDescriptor{Version: &branch, VersionType: &git.GitVersionTypeValues.Branch},
		IncludeContent:    &trueVal,
	})
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	reader := bufio.NewReader(output)
	// read the contents of the ReadCloser into a byte slice
	contents, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, err
	}
	return contents, http.StatusOK, nil
}

// GetRepositoryEnvironmentInfo on GitLab
func (client *AzureReposClient) GetRepositoryEnvironmentInfo(ctx context.Context, owner, repository, name string) (RepositoryEnvironmentInfo, error) {
	return RepositoryEnvironmentInfo{}, getUnsupportedInAzureError("get repository environment info")
}

func (client *AzureReposClient) GetModifiedFiles(ctx context.Context, _, repository, refBefore, refAfter string) ([]string, error) {
	if err := validateParametersNotBlank(map[string]string{
		"repository": repository,
		"refBefore":  refBefore,
		"refAfter":   refAfter,
	}); err != nil {
		return nil, err
	}

	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}

	fileNamesSet := datastructures.MakeSet[string]()
	changesToReturn := vcsutils.PointerOf(100)
	changesToSkip := vcsutils.PointerOf(0)

	for *changesToReturn >= 0 {
		commitDiffs, err := azureReposGitClient.GetCommitDiffs(ctx, git.GetCommitDiffsArgs{
			Top:                     changesToReturn,
			Skip:                    changesToSkip,
			RepositoryId:            &repository,
			Project:                 &client.vcsInfo.Project,
			DiffCommonCommit:        vcsutils.PointerOf(true),
			BaseVersionDescriptor:   &git.GitBaseVersionDescriptor{BaseVersion: &refBefore},
			TargetVersionDescriptor: &git.GitTargetVersionDescriptor{TargetVersion: &refAfter},
		})
		if err != nil {
			return nil, err
		}

		changes := vcsutils.DefaultIfNotNil(commitDiffs.Changes)
		if len(changes) < *changesToReturn {
			changesToReturn = vcsutils.PointerOf(-1)
		} else {
			changesToSkip = vcsutils.PointerOf(*changesToSkip + *changesToReturn)
		}

		for _, anyChange := range changes {
			change, err := vcsutils.RemapFields[git.GitChange](anyChange, "json")
			if err != nil {
				return nil, err
			}

			changedItem, err := vcsutils.RemapFields[git.GitItem](change.Item, "json")
			if err != nil {
				return nil, err
			}

			if vcsutils.DefaultIfNotNil(changedItem.GitObjectType) != git.GitObjectTypeValues.Blob {
				// We are not interested in the folders (trees) and other Git types.
				continue
			}

			// Azure returns all paths with '/' prefix. Other providers doesn't, so let's
			// remove the prefix here to produce output of the same format.
			fileNamesSet.Add(strings.TrimPrefix(vcsutils.DefaultIfNotNil(changedItem.Path), "/"))
		}
	}
	_ = fileNamesSet.Remove("") // Make sure there are no blank filepath.
	fileNamesList := fileNamesSet.ToSlice()
	sort.Strings(fileNamesList)
	return fileNamesList, nil
}

func (client *AzureReposClient) CreateBranch(ctx context.Context, owner, repository, sourceBranch, newBranch string) error {
	return getUnsupportedInAzureError("create branch")
}

func (client *AzureReposClient) AllowWorkflows(ctx context.Context, owner string) error {
	return getUnsupportedInAzureError("allow workflows")
}

func (client *AzureReposClient) AddOrganizationSecret(ctx context.Context, owner, secretName, secretValue string) error {
	return getUnsupportedInAzureError("add organization secret")
}

func (client *AzureReposClient) CreateOrgVariable(ctx context.Context, owner, variableName, variableValue string) error {
	return getUnsupportedInAzureError("create organization variable")
}

func (client *AzureReposClient) CommitAndPushFiles(ctx context.Context, owner, repo, sourceBranch, commitMessage, authorName, authorEmail string, files []FileToCommit) error {
	return getUnsupportedInAzureError("commit and push files")
}

func (client *AzureReposClient) GetRepoCollaborators(ctx context.Context, owner, repo, affiliation, permission string) ([]string, error) {
	return nil, getUnsupportedInAzureError("get repo collaborators")
}

func (client *AzureReposClient) GetRepoTeamsByPermissions(ctx context.Context, owner, repo string, permissions []string) ([]int64, error) {
	return nil, getUnsupportedInAzureError("get repo teams by permissions")
}

func (client *AzureReposClient) CreateOrUpdateEnvironment(ctx context.Context, owner, repo, envName string, teams []int64, users []string) error {
	return getUnsupportedInAzureError("create or update environment")
}

func (client *AzureReposClient) MergePullRequest(ctx context.Context, owner, repo string, prNumber int, commitMessage string) error {
	return getUnsupportedInAzureError("merge pull request")
}

func (client *AzureReposClient) CreatePullRequestDetailed(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) (CreatedPullRequestInfo, error) {
	return CreatedPullRequestInfo{}, getUnsupportedInAzureError("create pull request detailed")
}

func (client *AzureReposClient) UploadSnapshotToDependencyGraph(ctx context.Context, owner, repo string, snapshot *SbomSnapshot) error {
	return getUnsupportedInAzureError("uploading snapshot to dependency graph UI")
}

func parsePullRequestDetails(client *AzureReposClient, pullRequest git.GitPullRequest, owner, repository string, withBody bool) PullRequestInfo {
	// Trim the branches prefix and get the actual branches name
	shortSourceName := plumbing.ReferenceName(*pullRequest.SourceRefName).Short()
	shortTargetName := plumbing.ReferenceName(*pullRequest.TargetRefName).Short()

	var prBody string
	bodyPtr := pullRequest.Description
	if bodyPtr != nil && withBody {
		prBody = *bodyPtr
	}

	// When a pull request is from a forked repository, extract the owner.
	sourceRepoOwner := owner
	if pullRequest.ForkSource != nil {
		if sourceRepoOwner = extractOwnerFromForkedRepoUrl(pullRequest.ForkSource); sourceRepoOwner == "" {
			client.logger.Warn(vcsutils.FailedForkedRepositoryExtraction)
		}
	}
	return PullRequestInfo{
		ID:     int64(*pullRequest.PullRequestId),
		Title:  vcsutils.DefaultIfNotNil(pullRequest.Title),
		Body:   prBody,
		URL:    vcsutils.DefaultIfNotNil(pullRequest.Url),
		Author: vcsutils.DefaultIfNotNil(pullRequest.CreatedBy.DisplayName),
		Source: BranchInfo{
			Name:       shortSourceName,
			Repository: repository,
			Owner:      sourceRepoOwner,
		},
		Target: BranchInfo{
			Name:       shortTargetName,
			Repository: repository,
			Owner:      owner,
		},
	}
}

// Extract the repository owner of a forked source
func extractOwnerFromForkedRepoUrl(forkedGit *git.GitForkRef) string {
	if forkedGit == nil || forkedGit.Repository == nil || forkedGit.Repository.Url == nil {
		return ""
	}
	url := *forkedGit.Repository.Url
	if !strings.Contains(url, defaultAzureBaseUrl) {
		return ""
	}
	owner := strings.Split(strings.TrimPrefix(url, defaultAzureBaseUrl), "/")[0]
	return owner
}

// mapStatusToString maps commit status enum to string, specific for azure.
func mapStatusToString(status CommitStatus) string {
	conversionMap := map[CommitStatus]string{
		Pass:       "Succeeded",
		Fail:       "Failed",
		Error:      "Error",
		InProgress: "Pending",
	}
	return conversionMap[status]
}

func extractTimeFromAzuredevopsTime(rawStatus *azuredevops.Time) time.Time {
	if rawStatus == nil {
		return time.Time{}
	}
	return extractTimeWithFallback(&rawStatus.Time)
}

func azureMapPullRequestState(state vcsutils.PullRequestState) *git.PullRequestStatus {
	switch state {
	case vcsutils.Open:
		return &git.PullRequestStatusValues.Active
	case vcsutils.Closed:
		return &git.PullRequestStatusValues.Abandoned
	default:
		return nil
	}
}

func mapVoteToState(vote int) string {
	switch vote {
	case 10:
		return "APPROVED"
	case 5:
		return "APPROVED_WITH_SUGGESTIONS"
	case -5:
		return "CHANGES_REQUESTED"
	case -10:
		return "REJECTED"
	default:
		return "UNKNOWN"
	}
}
