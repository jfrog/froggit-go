package vcsclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/xanzy/go-gitlab"
)

// GitLabClient API version 4
type GitLabClient struct {
	glClient *gitlab.Client
}

// NewGitLabClient create a new GitLabClient
func NewGitLabClient(vcsInfo VcsInfo) (*GitLabClient, error) {
	var client *gitlab.Client
	var err error
	if vcsInfo.APIEndpoint != "" {
		client, err = gitlab.NewClient(vcsInfo.Token, gitlab.WithBaseURL(vcsInfo.APIEndpoint))
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

// TestConnection on GitLab
func (client *GitLabClient) TestConnection(ctx context.Context) error {
	_, _, err := client.glClient.Projects.ListProjects(nil, gitlab.WithContext(ctx))
	return err
}

// ListRepositories on GitLab
func (client *GitLabClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	simple := true
	results := make(map[string][]string)
	membership := true
	for pageID := 1; ; pageID++ {
		options := &gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{Page: pageID}, Simple: &simple, Membership: &membership}
		projects, response, err := client.glClient.Projects.ListProjects(options, gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		for _, project := range projects {
			owner := project.Namespace.Path
			results[owner] = append(results[owner], project.Path)
		}
		if pageID >= response.TotalPages {
			break
		}
	}
	return results, nil
}

// ListBranches on GitLab
func (client *GitLabClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	branches, _, err := client.glClient.Branches.ListBranches(getProjectID(owner, repository), nil,
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

// AddSshKeyToRepository on GitLab
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
	_, _, err = client.glClient.DeployKeys.AddDeployKey(getProjectID(owner, repository), options, gitlab.WithContext(ctx))
	return err
}

// CreateWebhook on GitLab
func (client *GitLabClient) CreateWebhook(ctx context.Context, owner, repository, branch, payloadURL string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	token := vcsutils.CreateToken()
	projectHook := createProjectHook(branch, payloadURL, webhookEvents...)
	options := &gitlab.AddProjectHookOptions{
		Token:                  &token,
		URL:                    &projectHook.URL,
		MergeRequestsEvents:    &projectHook.MergeRequestsEvents,
		PushEvents:             &projectHook.PushEvents,
		PushEventsBranchFilter: &projectHook.PushEventsBranchFilter,
	}
	response, _, err := client.glClient.Projects.AddProjectHook(getProjectID(owner, repository), options,
		gitlab.WithContext(ctx))
	if err != nil {
		return "", "", err
	}
	return strconv.Itoa(response.ID), token, nil
}

// UpdateWebhook on GitLab
func (client *GitLabClient) UpdateWebhook(ctx context.Context, owner, repository, branch, payloadURL, token,
	webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	projectHook := createProjectHook(branch, payloadURL, webhookEvents...)
	options := &gitlab.EditProjectHookOptions{
		Token:                  &token,
		URL:                    &projectHook.URL,
		MergeRequestsEvents:    &projectHook.MergeRequestsEvents,
		PushEvents:             &projectHook.PushEvents,
		PushEventsBranchFilter: &projectHook.PushEventsBranchFilter,
	}
	intWebhook, err := strconv.Atoi(webhookID)
	if err != nil {
		return err
	}
	_, _, err = client.glClient.Projects.EditProjectHook(getProjectID(owner, repository), intWebhook, options,
		gitlab.WithContext(ctx))
	return err
}

// DeleteWebhook on GitLab
func (client *GitLabClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	intWebhook, err := strconv.Atoi(webhookID)
	if err != nil {
		return err
	}
	_, err = client.glClient.Projects.DeleteProjectHook(getProjectID(owner, repository), intWebhook,
		gitlab.WithContext(ctx))
	return err
}

// SetCommitStatus on GitLab
func (client *GitLabClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository, ref,
	title, description, detailsURL string) error {
	options := &gitlab.SetCommitStatusOptions{
		State:       gitlab.BuildStateValue(getGitLabCommitState(commitStatus)),
		Ref:         &ref,
		Name:        &title,
		Description: &description,
		TargetURL:   &detailsURL,
	}
	_, _, err := client.glClient.Commits.SetCommitStatus(getProjectID(owner, repository), ref, options,
		gitlab.WithContext(ctx))
	return err
}

// DownloadRepository on GitLab
func (client *GitLabClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error {
	format := "tar.gz"
	options := &gitlab.ArchiveOptions{
		Format: &format,
		SHA:    &branch,
	}
	response, _, err := client.glClient.Repositories.Archive(getProjectID(owner, repository), options,
		gitlab.WithContext(ctx))
	if err != nil {
		return err
	}
	return vcsutils.Untar(localPath, bytes.NewReader(response), true)
}

// CreatePullRequest on GitLab
func (client *GitLabClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch,
	title, description string) error {
	options := &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		Description:  &description,
		SourceBranch: &sourceBranch,
		TargetBranch: &targetBranch,
	}
	_, _, err := client.glClient.MergeRequests.CreateMergeRequest(getProjectID(owner, repository), options,
		gitlab.WithContext(ctx))
	return err
}

// ListOpenPullRequests on GitLab
func (client *GitLabClient) ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	openState := "open"
	allScope := "all"
	options := &gitlab.ListMergeRequestsOptions{
		State: &openState,
		Scope: &allScope,
	}
	mergeRequests, _, err := client.glClient.MergeRequests.ListMergeRequests(options,
		gitlab.WithContext(ctx))
	if err != nil {
		return []PullRequestInfo{}, err
	}
	return mapGitLabMergeRequestToPullRequestInfoList(mergeRequests), nil
}

// AddPullRequestComment on GitLab
func (client *GitLabClient) AddPullRequestComment(ctx context.Context, owner, repository, content string, pullRequestID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "content": content})
	if err != nil {
		return err
	}
	options := &gitlab.CreateMergeRequestNoteOptions{
		Body: &content,
	}
	_, _, err = client.glClient.Notes.CreateMergeRequestNote(getProjectID(owner, repository), pullRequestID, options,
		gitlab.WithContext(ctx))

	return err
}

// ListPullRequestComments on GitLab
func (client *GitLabClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return []CommentInfo{}, err
	}
	commentsList, _, err := client.glClient.Notes.ListMergeRequestNotes(getProjectID(owner, repository), pullRequestID, &gitlab.ListMergeRequestNotesOptions{},
		gitlab.WithContext(ctx))
	if err != nil {
		return []CommentInfo{}, err
	}
	return mapGitLabNotesToCommentInfoList(commentsList), nil
}

// GetLatestCommit on GitLab
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

	commits, _, err := client.glClient.Commits.ListCommits(getProjectID(owner, repository), listOptions, gitlab.WithContext(ctx))
	if err != nil {
		return CommitInfo{}, err
	}
	if len(commits) > 0 {
		latestCommit := commits[0]
		return mapGitLabCommitToCommitInfo(latestCommit), nil
	}
	return CommitInfo{}, errors.New(`{"message":"404 Not Found"}`)
}

// GetRepositoryInfo on GitLab
func (client *GitLabClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return RepositoryInfo{}, err
	}

	project, _, err := client.glClient.Projects.GetProject(getProjectID(owner, repository), nil, gitlab.WithContext(ctx))
	if err != nil {
		return RepositoryInfo{}, err
	}

	return RepositoryInfo{CloneInfo: CloneInfo{HTTP: project.HTTPURLToRepo, SSH: project.SSHURLToRepo}}, nil
}

// GetCommitBySha on GitLab
func (client *GitLabClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"sha":        sha,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	commit, _, err := client.glClient.Commits.GetCommit(getProjectID(owner, repository), sha, gitlab.WithContext(ctx))
	if err != nil {
		return CommitInfo{}, err
	}
	return mapGitLabCommitToCommitInfo(commit), nil
}

// CreateLabel on GitLab
func (client *GitLabClient) CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "LabelInfo.name": labelInfo.Name})
	if err != nil {
		return err
	}

	_, _, err = client.glClient.Labels.CreateLabel(getProjectID(owner, repository), &gitlab.CreateLabelOptions{
		Name:        &labelInfo.Name,
		Description: &labelInfo.Description,
		Color:       &labelInfo.Color,
	}, gitlab.WithContext(ctx))

	return err
}

// GetLabel on GitLub
func (client *GitLabClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "name": name})
	if err != nil {
		return nil, err
	}

	labels, _, err := client.glClient.Labels.ListLabels(getProjectID(owner, repository), &gitlab.ListLabelsOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	for _, label := range labels {
		if label.Name == name {
			return &LabelInfo{
				Name:        label.Name,
				Description: label.Description,
				Color:       strings.TrimPrefix(label.Color, "#"),
			}, err
		}
	}

	return nil, nil
}

// ListPullRequestLabels on GitLab
func (client *GitLabClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return []string{}, err
	}
	mergeRequest, _, err := client.glClient.MergeRequests.GetMergeRequest(getProjectID(owner, repository), pullRequestID,
		&gitlab.GetMergeRequestsOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return []string{}, err
	}

	return mergeRequest.Labels, nil
}

// UnlabelPullRequest on GitLab
func (client *GitLabClient) UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return err
	}
	_, _, err = client.glClient.MergeRequests.UpdateMergeRequest(getProjectID(owner, repository), pullRequestID, &gitlab.UpdateMergeRequestOptions{
		RemoveLabels: gitlab.Labels{name},
	}, gitlab.WithContext(ctx))
	return err
}

func (client *GitLabClient) UploadCodeScanning(ctx context.Context, owner string, repository string, branch string, scan string) (string, error) {
	return "", errCodeScanningNotSupported
}

func getProjectID(owner, project string) string {
	return fmt.Sprintf("%s/%s", owner, project)
}

func createProjectHook(branch string, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) *gitlab.ProjectHook {
	options := &gitlab.ProjectHook{URL: payloadURL}
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

func mapGitLabNotesToCommentInfoList(notes []*gitlab.Note) (res []CommentInfo) {
	for _, note := range notes {
		res = append(res, CommentInfo{
			ID:      int64(note.ID),
			Content: note.Body,
			Created: *note.CreatedAt,
		})
	}
	return
}

func mapGitLabMergeRequestToPullRequestInfoList(mergeRequests []*gitlab.MergeRequest) (res []PullRequestInfo) {
	for _, mergeRequest := range mergeRequests {
		res = append(res, PullRequestInfo{
			ID:     int64(mergeRequest.IID),
			Source: BranchInfo{Name: mergeRequest.SourceBranch},
			Target: BranchInfo{Name: mergeRequest.TargetBranch},
		})
	}
	return
}
