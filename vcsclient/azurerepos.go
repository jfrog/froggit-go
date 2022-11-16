package vcsclient

import (
	"context"
	"fmt"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"io"
	"net/http"
	"os"
	"strings"
)

type AzureReposClient struct {
	vcsInfo             VcsInfo
	azureReposGitClient *git.Client
	connectionDetails   *azuredevops.Connection
}

func NewAzureReposClient(vcsInfo VcsInfo) (*AzureReposClient, error) {
	client := &AzureReposClient{vcsInfo: vcsInfo}
	if err := client.buildAzureReposClient(context.Background()); err != nil {
		return nil, err
	}
	return client, nil
}

func (client *AzureReposClient) buildAzureReposClient(ctx context.Context) error {
	baseUrl := strings.TrimSuffix(client.vcsInfo.APIEndpoint, string(os.PathSeparator))
	client.connectionDetails = azuredevops.NewPatConnection(baseUrl, client.vcsInfo.Token)
	azureGitClient, err := git.NewClient(ctx, client.connectionDetails)
	client.azureReposGitClient = &azureGitClient
	return err
}

func (client *AzureReposClient) TestConnection(ctx context.Context) error {
	buildClient := azuredevops.NewClient(client.connectionDetails, client.connectionDetails.BaseUrl)
	_, err := buildClient.GetResourceAreas(ctx)
	return err
}

func (client *AzureReposClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	repositories := make(map[string][]string)
	resp, err := (*client.azureReposGitClient).GetRepositories(ctx, git.GetRepositoriesArgs{Project: &client.vcsInfo.Project})
	if err != nil {
		return repositories, err
	}
	for _, repo := range *resp {
		repositories[client.vcsInfo.Project] = append(repositories[client.vcsInfo.Project], *repo.Name)
	}
	return repositories, nil
}

func (client *AzureReposClient) ListBranches(ctx context.Context, _, repository string) ([]string, error) {
	var branches []string
	gitBranchStats, err := (*client.azureReposGitClient).GetBranches(ctx, git.GetBranchesArgs{Project: &client.vcsInfo.Project, RepositoryId: &repository})
	if err != nil {
		return nil, err
	}
	for _, branch := range *gitBranchStats {
		branches = append(branches, *branch.Name)
	}
	return branches, nil
}

func (client *AzureReposClient) DownloadRepository(ctx context.Context, _, repository, branch, localPath string) (err error) {
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

	var zipRepo *os.File
	if zipRepo, err = os.Create(fmt.Sprintf("%s_azure_repo.zip", repository)); err != nil {
		return
	}
	defer func() {
		e := zipRepo.Close()
		if err == nil {
			err = e
		}
	}()
	res, err := client.sendDownloadRepoRequest(ctx, repository, branch)
	defer func() {
		e := res.Body.Close()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return
	}
	// Copy downloaded repository to zipRepo zip file.
	if _, err = io.Copy(zipRepo, res.Body); err != nil {
		return
	}
	err = vcsutils.Unzip(zipRepo.Name(), localPath)
	return
}

func (client *AzureReposClient) sendDownloadRepoRequest(ctx context.Context, repository string, branch string) (res *http.Response, err error) {
	downloadRepoUrl := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items/items?path=/&[…]ptor[version]=%s&$format=zip",
		client.connectionDetails.BaseUrl,
		client.vcsInfo.Project,
		repository,
		branch)
	headers := map[string]string{
		"Authorization": client.connectionDetails.AuthorizationString,
		"download":      "true",
		"resolveLfs":    "true",
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
	if statusOk := res.StatusCode >= 200 && res.StatusCode < 300; !statusOk {
		err = fmt.Errorf("bad HTTP status: %d", res.StatusCode)
	}
	return
}

func (client *AzureReposClient) CreatePullRequest(ctx context.Context, _, repository, sourceBranch, targetBranch, title, description string) error {
	sourceBranch = vcsutils.AddBranchPrefix(sourceBranch)
	targetBranch = vcsutils.AddBranchPrefix(targetBranch)
	_, err := (*client.azureReposGitClient).CreatePullRequest(ctx, git.CreatePullRequestArgs{
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

func (client *AzureReposClient) AddPullRequestComment(ctx context.Context, _, repository, content string, pullRequestID int) error {
	// To add a new comment to the pull request, we need to open a new thread, and add a comment inside this thread.
	_, err := (*client.azureReposGitClient).CreateThread(ctx, git.CreateThreadArgs{
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
	threads, err := (*client.azureReposGitClient).GetThreads(ctx, git.GetThreadsArgs{
		RepositoryId:  &repository,
		PullRequestId: &pullRequestID,
		Project:       &client.vcsInfo.Project,
	})
	if err != nil {
		return nil, err
	}
	var commentInfo []CommentInfo
	var commentsAggregator strings.Builder
	for _, thread := range *threads {
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
		commentsAggregator.Reset()
	}
	return commentInfo, nil
}

func (client *AzureReposClient) ListOpenPullRequests(ctx context.Context, _, repository string) ([]PullRequestInfo, error) {
	pullRequests, err := (*client.azureReposGitClient).GetPullRequests(ctx, git.GetPullRequestsArgs{
		RepositoryId:   &repository,
		Project:        &client.vcsInfo.Project,
		SearchCriteria: &git.GitPullRequestSearchCriteria{Status: &git.PullRequestStatusValues.Active},
	})
	if err != nil {
		return nil, err
	}
	var pullRequestsInfo []PullRequestInfo
	for _, pullRequest := range *pullRequests {
		pullRequestsInfo = append(pullRequestsInfo, PullRequestInfo{
			ID: int64(*pullRequest.PullRequestId),
			Source: BranchInfo{
				Name:       vcsutils.DefaultIfNotNil(pullRequest.SourceRefName),
				Repository: repository,
			},
			Target: BranchInfo{
				Name:       vcsutils.DefaultIfNotNil(pullRequest.TargetRefName),
				Repository: repository,
			},
		})
	}
	return pullRequestsInfo, nil
}

func (client *AzureReposClient) GetLatestCommit(ctx context.Context, _, repository, branch string) (CommitInfo, error) {
	latestCommitInfo := CommitInfo{}
	commits, err := (*client.azureReposGitClient).GetCommits(ctx, git.GetCommitsArgs{
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

func (client *AzureReposClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission Permission) error {
	return errAddSshKeyToRepositoryNotSupported
}

func (client *AzureReposClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	return RepositoryInfo{}, errGetRepositoryInfoNotSupported
}

func (client *AzureReposClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	return CommitInfo{}, errGetCommitByShaNotSupported
}

func (client *AzureReposClient) CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error {
	return errCreateLabelNotSupported
}

func (client *AzureReposClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	return nil, errGetLabelNotSupported
}

func (client *AzureReposClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	return nil, errListPullRequestLabelsNotSupported
}

func (client *AzureReposClient) UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error {
	return errUnlabelPullRequestNotSupported
}

func (client *AzureReposClient) UploadCodeScanning(ctx context.Context, owner, repository, branch, scanResults string) (string, error) {
	return "", errUploadCodeScanningNotSupported
}

func (client *AzureReposClient) CreateWebhook(ctx context.Context, owner, repository, branch, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	return "", "", errCreateWebhookNotSupported
}

func (client *AzureReposClient) UpdateWebhook(ctx context.Context, owner, repository, branch, payloadURL, token, webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	return errUpdateWebhookNotSupported
}

func (client *AzureReposClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	return errDeleteWebhookNotSupported
}

func (client *AzureReposClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref, title, description, detailsURL string) error {
	return errSetCommitStatusNotSupported
}
