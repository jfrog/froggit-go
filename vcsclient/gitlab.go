package vcsclient

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/xanzy/go-gitlab"
)

type GitLabClient struct {
	glClient *gitlab.Client
}

func NewGitLabClient(vcsInfo VcsInfo) (*GitLabClient, error) {
	var client *gitlab.Client
	var err error
	if vcsInfo.ApiEndpoint != "" {
		client, err = gitlab.NewClient(vcsInfo.Token, gitlab.WithBaseURL(vcsInfo.ApiEndpoint))
	} else {
		client, err = gitlab.NewClient(vcsInfo.Token)
	}
	if err != nil {
		return nil, err
	}

	return &GitLabClient{
		glClient: client,
	}, nil
}

func (client *GitLabClient) TestConnection(ctx context.Context) error {
	_, _, err := client.glClient.Projects.ListProjects(nil, gitlab.WithContext(ctx))
	return err
}

func (client *GitLabClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	simple := true
	results := make(map[string][]string)
	for pageId := 1; ; pageId++ {
		options := &gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{Page: pageId}, Simple: &simple}
		projects, response, err := client.glClient.Projects.ListProjects(options, gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		for _, project := range projects {
			owner := project.Namespace.Path
			results[owner] = append(results[owner], project.Path)
		}
		if pageId >= response.TotalPages {
			break
		}
	}
	return results, nil
}

func (client *GitLabClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	branches, _, err := client.glClient.Branches.ListBranches(getProjectId(owner, repository), nil,
		gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(branches))
	for _, branch := range branches {
		results = append(results, branch.Name)
	}
	return results, nil
}

func (client *GitLabClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission Permission) error {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"key name":   keyName,
		"public key": publicKey,
	})
	if err != nil {
		return err
	}

	canPush := false
	if permission == ReadWrite {
		canPush = true
	}
	options := &gitlab.AddDeployKeyOptions{
		Title:   &keyName,
		Key:     &publicKey,
		CanPush: &canPush,
	}
	_, _, err = client.glClient.DeployKeys.AddDeployKey(getProjectId(owner, repository), options, gitlab.WithContext(ctx))
	return err
}

func (client *GitLabClient) CreateWebhook(ctx context.Context, owner, repository, branch, payloadUrl string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	token := vcsutils.CreateToken()
	projectHook := createProjectHook(branch, payloadUrl, webhookEvents...)
	options := &gitlab.AddProjectHookOptions{
		Token:                  &token,
		URL:                    &projectHook.URL,
		MergeRequestsEvents:    &projectHook.MergeRequestsEvents,
		PushEvents:             &projectHook.PushEvents,
		PushEventsBranchFilter: &projectHook.PushEventsBranchFilter,
	}
	response, _, err := client.glClient.Projects.AddProjectHook(getProjectId(owner, repository), options,
		gitlab.WithContext(ctx))
	if err != nil {
		return "", "", err
	}
	return strconv.Itoa(response.ID), token, nil
}

func (client *GitLabClient) UpdateWebhook(ctx context.Context, owner, repository, branch, payloadUrl, token,
	webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
	projectHook := createProjectHook(branch, payloadUrl, webhookEvents...)
	options := &gitlab.EditProjectHookOptions{
		Token:                  &token,
		URL:                    &projectHook.URL,
		MergeRequestsEvents:    &projectHook.MergeRequestsEvents,
		PushEvents:             &projectHook.PushEvents,
		PushEventsBranchFilter: &projectHook.PushEventsBranchFilter,
	}
	intWebhook, err := strconv.Atoi(webhookId)
	if err != nil {
		return err
	}
	_, _, err = client.glClient.Projects.EditProjectHook(getProjectId(owner, repository), intWebhook, options,
		gitlab.WithContext(ctx))
	return err
}

func (client *GitLabClient) DeleteWebhook(ctx context.Context, owner, repository, webhookId string) error {
	intWebhook, err := strconv.Atoi(webhookId)
	if err != nil {
		return err
	}
	_, err = client.glClient.Projects.DeleteProjectHook(getProjectId(owner, repository), intWebhook,
		gitlab.WithContext(ctx))
	return err
}

func (client *GitLabClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref,
	title, description, detailsUrl string) error {
	options := &gitlab.SetCommitStatusOptions{
		State:       gitlab.BuildStateValue(getGitLabCommitState(commitStatus)),
		Ref:         &ref,
		Name:        &title,
		Description: &description,
		TargetURL:   &detailsUrl,
	}
	_, _, err := client.glClient.Commits.SetCommitStatus(getProjectId(owner, repository), ref, options,
		gitlab.WithContext(ctx))
	return err
}

func (client *GitLabClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error {
	format := "tar.gz"
	options := &gitlab.ArchiveOptions{
		Format: &format,
		SHA:    &branch,
	}
	response, _, err := client.glClient.Repositories.Archive(getProjectId(owner, repository), options,
		gitlab.WithContext(ctx))
	if err != nil {
		return err
	}
	return vcsutils.Untar(localPath, bytes.NewReader(response), true)
}

func (client *GitLabClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch,
	title, description string) error {
	options := &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		Description:  &description,
		SourceBranch: &sourceBranch,
		TargetBranch: &targetBranch,
	}
	_, _, err := client.glClient.MergeRequests.CreateMergeRequest(getProjectId(owner, repository), options,
		gitlab.WithContext(ctx))
	return err
}

func (client *GitLabClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"branch":     branch,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	listOptions := &gitlab.ListCommitsOptions{
		RefName: &branch,
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 1,
		},
	}

	commits, _, err := client.glClient.Commits.ListCommits(getProjectId(owner, repository), listOptions, gitlab.WithContext(ctx))
	if err != nil {
		return CommitInfo{}, err
	}
	if len(commits) > 0 {
		latestCommit := commits[0]
		return mapGitLabCommitToCommitInfo(latestCommit), nil
	}
	return CommitInfo{}, nil
}

func (client *GitLabClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return RepositoryInfo{}, err
	}

	project, _, err := client.glClient.Projects.GetProject(getProjectId(owner, repository), nil, gitlab.WithContext(ctx))
	if err != nil {
		return RepositoryInfo{}, err
	}

	return RepositoryInfo{CloneInfo: CloneInfo{HTTP: project.HTTPURLToRepo, SSH: project.SSHURLToRepo}}, nil
}

func (client *GitLabClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"sha":        sha,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	commit, _, err := client.glClient.Commits.GetCommit(getProjectId(owner, repository), sha, gitlab.WithContext(ctx))
	if err != nil {
		return CommitInfo{}, err
	}
	return mapGitLabCommitToCommitInfo(commit), nil
}

func getProjectId(owner, project string) string {
	return fmt.Sprintf("%s/%s", owner, project)
}

func createProjectHook(branch string, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) *gitlab.ProjectHook {
	options := &gitlab.ProjectHook{URL: payloadUrl}
	for _, webhookEvent := range webhookEvents {
		switch webhookEvent {
		case vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrRejected, vcsutils.PrMerged:
			options.MergeRequestsEvents = true
		case vcsutils.Push:
			options.PushEvents = true
			options.PushEventsBranchFilter = branch
		}
	}
	return options
}

func getGitLabCommitState(commitState CommitStatus) string {
	switch commitState {
	case Pass:
		return "success"
	case Fail:
		return "failed"
	case Error:
		return "failed"
	case InProgress:
		return "running"
	}
	return ""
}

func mapGitLabCommitToCommitInfo(commit *gitlab.Commit) CommitInfo {
	return CommitInfo{
		Hash:          commit.ID,
		AuthorName:    commit.AuthorName,
		CommitterName: commit.CommitterName,
		Url:           commit.WebURL,
		Timestamp:     commit.CommittedDate.UTC().Unix(),
		Message:       commit.Message,
		ParentHashes:  commit.ParentIDs,
	}
}
