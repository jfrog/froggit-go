package vcsclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/mitchellh/mapstructure"

	"github.com/jfrog/froggit-go/vcsutils"
)

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
		e := os.Chdir(wd)
		if err == nil {
			err = e
		}
	}()
	res, err := client.sendDownloadRepoRequest(ctx, repository, branch)
	defer func() {
		if res.Body != nil {
			e := res.Body.Close()
			if err == nil {
				err = e
			}
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
		var commentsAggregator strings.Builder
		for _, comment := range *thread.Comments {
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

// ListOpenPullRequests on Azure Repos
func (client *AzureReposClient) ListOpenPullRequests(ctx context.Context, _, repository string) ([]PullRequestInfo, error) {
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
		// Trim the branches prefix and get the actual branches name
		shortSourceName := (*pullRequest.SourceRefName)[strings.LastIndex(*pullRequest.SourceRefName, "/")+1:]
		shortTargetName := (*pullRequest.TargetRefName)[strings.LastIndex(*pullRequest.TargetRefName, "/")+1:]
		pullRequestsInfo = append(pullRequestsInfo, PullRequestInfo{
			ID: int64(*pullRequest.PullRequestId),
			Source: BranchInfo{
				Name:       shortSourceName,
				Repository: repository,
			},
			Target: BranchInfo{
				Name:       shortTargetName,
				Repository: repository,
			},
		})
	}
	return pullRequestsInfo, nil
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
	return getUnsupportedInAzureError("set commit status")
}

// DownloadFileFromRepo on Azure Repos
func (client *AzureReposClient) DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) ([]byte, int, error) {
	return nil, 0, getUnsupportedInAzureError("download file from repo")
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
			change, err := remapFields[git.GitChange](anyChange, "json")
			if err != nil {
				return nil, err
			}

			changedItem, err := remapFields[git.GitItem](change.Item, "json")
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

// remapFields creates an instance of the T type and copies data from src parameter to it
// by mapping fields based on the tags with tagName (if not provided 'mapstructure' tag is used).
func remapFields[T any](src any, tagName string) (T, error) {
	var dst T
	if changeDecoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: tagName,
		Result:  &dst,
	}); err != nil {
		return dst, err
	} else if err := changeDecoder.Decode(src); err != nil {
		return dst, err
	}
	return dst, nil
}
