package vcsclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const defaultAzureBaseUrl = "https://dev.azure.com/"

// Azure Devops API version 6
type AzureReposClient struct {
	vcsInfo           VcsInfo
	connectionDetails *azuredevops.Connection
	logger            Log
}

// NewAzureReposClient create a new AzureReposClient
func NewAzureReposClient(vcsInfo VcsInfo, logger Log) (*AzureReposClient, error) {
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
	client.logger.Info(successfulRepoExtraction)
	// Generate .git folder with remote details
	return vcsutils.CreateDotGitFolderWithRemote(
		localPath,
		vcsutils.RemoteName,
		fmt.Sprintf("https://%s@%s/%s/_git/%s", owner, strings.TrimPrefix(client.connectionDetails.BaseUrl, "https://"), client.vcsInfo.Project, repository))
}

func (client *AzureReposClient) sendDownloadRepoRequest(ctx context.Context, repository string, branch string) (res *http.Response, err error) {
	downloadRepoUrl := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items/items?path=/&versionDescriptor[version]=%s&$format=zip",
		client.connectionDetails.BaseUrl,
		client.vcsInfo.Project,
		repository,
		branch)
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
	client.logger.Info(repository, successfulRepoDownload)
	return
}

// CreatePullRequest on Azure Repos
func (client *AzureReposClient) CreatePullRequest(ctx context.Context, _, repository, sourceBranch, targetBranch, title, description string) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	sourceBranch = vcsutils.AddBranchPrefix(sourceBranch)
	targetBranch = vcsutils.AddBranchPrefix(targetBranch)
	client.logger.Debug(creatingPullRequest, title)
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
func (client *AzureReposClient) UpdatePullRequest(ctx context.Context, owner, repository, title, body, targetBranchName string, prId int, state vcsutils.PullRequestState) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	// If the string is empty,do not add a prefix,as it indicates that the user does not intend to update the target branch.
	if targetBranchName != "" {
		targetBranchName = vcsutils.AddBranchPrefix(targetBranchName)
	}
	client.logger.Debug(updatingPullRequest, prId)
	_, err = azureReposGitClient.UpdatePullRequest(ctx, git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: &git.GitPullRequest{
			Description:   &body,
			Status:        azureMapPullRequestState(state),
			TargetRefName: &targetBranchName,
			Title:         &title,
		},
		RepositoryId:  &repository,
		PullRequestId: &prId,
		Project:       &client.vcsInfo.Project,
	})
	return err
}

// AddPullRequestComment on Azure Repos
func (client *AzureReposClient) AddPullRequestComment(ctx context.Context, _, repository, content string, pullRequestID int) error {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	// To add a new comment to the pull request, we need to open a new thread, and add a comment inside this thread.
	_, err = azureReposGitClient.CreateThread(ctx, git.CreateThreadArgs{
		CommentThread: &git.GitPullRequestCommentThread{
			Comments: &[]git.Comment{{Content: &content}},
			Status:   &git.CommentThreadStatusValues.Active,
		},
		RepositoryId:  &repository,
		PullRequestId: &pullRequestID,
		Project:       &client.vcsInfo.Project,
	})
	return err
}

// ListPullRequestComments returns all the pull request threads with their comments.
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
	client.logger.Debug(fetchingOpenPullRequests, repository)
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

func (client *AzureReposClient) GetPullRequestByID(ctx context.Context, owner, repository string, pullRequestId int) (pullRequestInfo PullRequestInfo, err error) {
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return
	}
	client.logger.Debug(fetchingPullRequestById, repository)
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
	azureReposGitClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}
	latestCommitInfo := CommitInfo{}
	commits, err := azureReposGitClient.GetCommits(ctx, git.GetCommitsArgs{
		RepositoryId:   &repository,
		Project:        &client.vcsInfo.Project,
		SearchCriteria: &git.GitQueryCommitsCriteria{ItemVersion: &git.GitVersionDescriptor{Version: &branch, VersionType: &git.GitVersionTypeValues.Branch}},
	})
	if err != nil {
		return latestCommitInfo, err
	}
	if len(*commits) > 0 {
		// The latest commit is the first in the list
		latestCommit := (*commits)[0]
		latestCommitInfo = CommitInfo{
			Hash:          vcsutils.DefaultIfNotNil(latestCommit.CommitId),
			AuthorName:    vcsutils.DefaultIfNotNil(latestCommit.Author.Name),
			CommitterName: vcsutils.DefaultIfNotNil(latestCommit.Committer.Name),
			Url:           vcsutils.DefaultIfNotNil(latestCommit.Url),
			Timestamp:     latestCommit.Committer.Date.Time.Unix(),
			Message:       vcsutils.DefaultIfNotNil(latestCommit.Comment),
			ParentHashes:  vcsutils.DefaultIfNotNil(latestCommit.Parents),
		}
	}
	return latestCommitInfo, nil
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
	return RepositoryInfo{}, getUnsupportedInAzureError("get repository info")
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

func parsePullRequestDetails(client *AzureReposClient, pullRequest git.GitPullRequest, owner, repository string, withBody bool) PullRequestInfo {
	// Trim the branches prefix and get the actual branches name
	shortSourceName := (*pullRequest.SourceRefName)[strings.LastIndex(*pullRequest.SourceRefName, "/")+1:]
	shortTargetName := (*pullRequest.TargetRefName)[strings.LastIndex(*pullRequest.TargetRefName, "/")+1:]

	var prBody string
	bodyPtr := pullRequest.Description
	if bodyPtr != nil && withBody {
		prBody = *bodyPtr
	}

	// When a pull request is from a forked repository,extract the owner.
	sourceRepoOwner := owner
	if pullRequest.ForkSource != nil {
		if sourceRepoOwner = extractOwnerFromForkedRepoUrl(pullRequest.ForkSource); sourceRepoOwner == "" {
			client.logger.Warn(failedForkedRepositoryExtraction)
		}
	}

	return PullRequestInfo{
		ID:   int64(*pullRequest.PullRequestId),
		Body: prBody,
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
