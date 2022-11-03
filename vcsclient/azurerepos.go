package vcsclient

import (
	"context"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"strings"
)

type AzureReposClient struct {
	vcsInfo VcsInfo
}

func NewAzureReposClient(vcsInfo VcsInfo) (*AzureReposClient, error) {
	return &AzureReposClient{vcsInfo: vcsInfo}, nil
}

func (client *AzureReposClient) buildAzureReposClient(ctx context.Context) (*git.Client, error) {
	baseUrl := strings.TrimSuffix(client.vcsInfo.APIEndpoint, "/") + "/" + client.vcsInfo.Username
	connection := azuredevops.NewPatConnection(baseUrl, client.vcsInfo.Token)
	azureGitClient, err := git.NewClient(ctx, connection)
	return &azureGitClient, err
}

// TestConnection on Azure Repos
func (client *AzureReposClient) TestConnection(ctx context.Context) error {
	azureReposClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	// TODO: Find API to check connection
	azureReposClient = azureReposClient
	return err
}

func (client *AzureReposClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	repositories := make(map[string][]string)
	azureReposClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return repositories, err
	}
	resp, err := (*azureReposClient).GetRepositories(ctx, git.GetRepositoriesArgs{Project: &client.vcsInfo.Project})
	if err != nil {
		return repositories, err
	}
	for _, repo := range *resp {
		repositories[client.vcsInfo.Project] = append(repositories[client.vcsInfo.Project], *repo.Name)
	}
	return repositories, nil
}

func (client *AzureReposClient) ListBranches(ctx context.Context, _, repository string) ([]string, error) {
	azureReposClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return nil, err
	}
	var branches []string
	gitBranchStats, err := (*azureReposClient).GetBranches(ctx, git.GetBranchesArgs{Project: &client.vcsInfo.Project, RepositoryId: &repository})
	if err != nil {
		return nil, err
	}
	for _, branch := range *gitBranchStats {
		branches = append(branches, *branch.Name)
	}
	return branches, nil
}

func (client *AzureReposClient) CreateWebhook(ctx context.Context, owner, repository, branch, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) UpdateWebhook(ctx context.Context, owner, repository, branch, payloadURL, token, webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref, title, description, detailsURL string) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) DownloadRepository(ctx context.Context, _, repository, branch, localPath string) error {
	azureReposClient, err := client.buildAzureReposClient(ctx)
	if err != nil {
		return err
	}
	clientRepoDetails, err := (*azureReposClient).GetRepository(ctx, git.GetRepositoryArgs{
		RepositoryId: &repository,
		Project:      &client.vcsInfo.Project,
	})
	downloadedRepo, err := gogit.PlainClone(localPath, false, &gogit.CloneOptions{URL: *clientRepoDetails.Url, ReferenceName: plumbing.NewBranchReferenceName(branch)})
	if err != nil {
		return err
	}
	downloadedRepo = downloadedRepo
	return nil
}

func (client *AzureReposClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) AddPullRequestComment(ctx context.Context, owner, repository, content string, pullRequestID int) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission Permission) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error {
	//TODO implement me
	panic("implement me")
}

func (client *AzureReposClient) UploadCodeScanning(ctx context.Context, owner, repository, branch, scanResults string) (string, error) {
	//TODO implement me
	panic("implement me")
}
