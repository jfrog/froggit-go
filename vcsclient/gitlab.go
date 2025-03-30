package vcsclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/xanzy/go-gitlab"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitLabClient API version 4
type GitLabClient struct {
	glClient *gitlab.Client
	vcsInfo  VcsInfo
	logger   vcsutils.Log
}

// NewGitLabClient create a new GitLabClient
func NewGitLabClient(vcsInfo VcsInfo, logger vcsutils.Log) (*GitLabClient, error) {
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
		vcsInfo:  vcsInfo,
		logger:   logger,
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
		TagPushEvents:          &projectHook.TagPushEvents,
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
		TagPushEvents:          &projectHook.TagPushEvents,
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

// GetCommitStatuses on GitLab
func (client *GitLabClient) GetCommitStatuses(ctx context.Context, _, repository, ref string) (status []CommitStatusInfo, err error) {
	statuses, _, err := client.glClient.Commits.GetCommitStatuses(repository, ref, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	results := make([]CommitStatusInfo, 0)
	for _, singleStatus := range statuses {
		results = append(results, CommitStatusInfo{
			State:         commitStatusAsStringToStatus(singleStatus.Status),
			Description:   singleStatus.Description,
			DetailsUrl:    singleStatus.TargetURL,
			Creator:       singleStatus.Author.Name,
			LastUpdatedAt: extractTimeWithFallback(singleStatus.FinishedAt),
			CreatedAt:     extractTimeWithFallback(singleStatus.CreatedAt),
		})
	}
	return results, nil
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
	client.logger.Info(repository, vcsutils.SuccessfulRepoDownload)
	err = vcsutils.Untar(localPath, bytes.NewReader(response), true)
	if err != nil {
		return err
	}

	repositoryInfo, err := client.GetRepositoryInfo(ctx, owner, repository)
	if err != nil {
		return err
	}

	client.logger.Info(vcsutils.SuccessfulRepoExtraction)
	return vcsutils.CreateDotGitFolderWithRemote(localPath, vcsutils.RemoteName, repositoryInfo.CloneInfo.HTTP)
}

func (client *GitLabClient) GetPullRequestCommentSizeLimit() int {
	return gitlabMergeRequestCommentSizeLimit
}

func (client *GitLabClient) GetPullRequestDetailsSizeLimit() int {
	return gitlabMergeRequestDetailsSizeLimit
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
	client.logger.Debug("creating new merge request:", title)
	_, _, err := client.glClient.MergeRequests.CreateMergeRequest(getProjectID(owner, repository), options,
		gitlab.WithContext(ctx))
	return err
}

// UpdatePullRequest on GitLab
func (client *GitLabClient) UpdatePullRequest(ctx context.Context, owner, repository, title, body, targetBranchName string, prId int, state vcsutils.PullRequestState) error {
	options := &gitlab.UpdateMergeRequestOptions{
		Title:        &title,
		Description:  &body,
		TargetBranch: &targetBranchName,
		StateEvent:   mapGitLabPullRequestState(&state),
	}
	client.logger.Debug("updating details of merge request ID:", prId)
	_, _, err := client.glClient.MergeRequests.UpdateMergeRequest(getProjectID(owner, repository), prId, options, gitlab.WithContext(ctx))
	return err
}

// ListOpenPullRequestsWithBody on GitLab
func (client *GitLabClient) ListOpenPullRequestsWithBody(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	return client.getOpenPullRequests(ctx, owner, repository, true)
}

// ListOpenPullRequests on GitLab
func (client *GitLabClient) ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	return client.getOpenPullRequests(ctx, owner, repository, false)
}

func (client *GitLabClient) getOpenPullRequests(ctx context.Context, owner, repository string, withBody bool) ([]PullRequestInfo, error) {
	openState := "opened"
	allScope := "all"
	options := &gitlab.ListProjectMergeRequestsOptions{
		State: &openState,
		Scope: &allScope,
	}
	mergeRequests, _, err := client.glClient.MergeRequests.ListProjectMergeRequests(getProjectID(owner, repository), options, gitlab.WithContext(ctx))
	if err != nil {
		return []PullRequestInfo{}, err
	}
	return client.mapGitLabMergeRequestToPullRequestInfoList(mergeRequests, owner, repository, withBody)
}

// GetPullRequestInfoById on GitLab
func (client *GitLabClient) GetPullRequestByID(_ context.Context, owner, repository string, pullRequestId int) (pullRequestInfo PullRequestInfo, err error) {
	client.logger.Debug("fetching merge requests by ID in", repository)
	mergeRequest, glResponse, err := client.glClient.MergeRequests.GetMergeRequest(getProjectID(owner, repository), pullRequestId, nil)
	if err != nil {
		return PullRequestInfo{}, err
	}
	if glResponse != nil {
		if err = vcsutils.CheckResponseStatusWithBody(glResponse.Response, http.StatusOK); err != nil {
			return PullRequestInfo{}, err
		}
	}
	pullRequestInfo, err = client.mapGitLabMergeRequestToPullRequestInfo(mergeRequest, false, owner, repository)
	return
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

// AddPullRequestReviewComments adds comments to a pull request on GitLab.
func (client *GitLabClient) AddPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...PullRequestComment) error {
	// Validate parameters
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository}); err != nil {
		return err
	}

	// Check if comments are provided
	if len(comments) == 0 {
		return errors.New("could not add merge request review comments, no comments provided")
	}

	projectID := getProjectID(owner, repository)

	// Get merge request diff versions
	versions, err := client.getMergeRequestDiffVersions(ctx, projectID, pullRequestID)
	if err != nil {
		return fmt.Errorf("could not get merge request diff versions: %w", err)
	}

	// Get merge request details
	mergeRequestChanges, err := client.getMergeRequestDiff(ctx, projectID, pullRequestID)
	if err != nil {
		return fmt.Errorf("could not get merge request changes: %w", err)
	}

	for _, comment := range comments {
		if err = client.addPullRequestReviewComment(ctx, projectID, pullRequestID, comment, versions, mergeRequestChanges); err != nil {
			return err
		}
	}

	return nil
}

func (client *GitLabClient) getMergeRequestDiffVersions(ctx context.Context, projectID string, pullRequestID int) ([]*gitlab.MergeRequestDiffVersion, error) {
	versions, _, err := client.glClient.MergeRequests.GetMergeRequestDiffVersions(projectID, pullRequestID, &gitlab.GetMergeRequestDiffVersionsOptions{}, gitlab.WithContext(ctx))
	return versions, err
}

func (client *GitLabClient) getMergeRequestDiff(ctx context.Context, projectID string, pullRequestID int) ([]*gitlab.MergeRequestDiff, error) {
	mergeRequestChanges, _, err := client.glClient.MergeRequests.ListMergeRequestDiffs(projectID, pullRequestID, nil, gitlab.WithContext(ctx))
	return mergeRequestChanges, err
}

func (client *GitLabClient) addPullRequestReviewComment(ctx context.Context, projectID string, pullRequestID int, comment PullRequestComment, versions []*gitlab.MergeRequestDiffVersion, mergeRequestChanges []*gitlab.MergeRequestDiff) error {
	// Find the corresponding change in merge request
	var newPath, oldPath string
	var newLine int
	var diffFound bool

	for _, diff := range mergeRequestChanges {
		if diff.NewPath != comment.NewFilePath {
			continue
		}

		diffFound = true
		newLine = comment.NewStartLine
		newPath = diff.NewPath

		// New files don't have old data
		if !diff.NewFile {
			oldPath = diff.OldPath
		}
		break
	}

	// If no matching change is found, return an error
	if !diffFound {
		return fmt.Errorf("could not find changes to %s in the current merge request", comment.NewFilePath)
	}

	// Create a NotePosition for the comment
	latestVersion := versions[0]
	diffPosition := &gitlab.PositionOptions{
		StartSHA:     &latestVersion.StartCommitSHA,
		HeadSHA:      &latestVersion.HeadCommitSHA,
		BaseSHA:      &latestVersion.BaseCommitSHA,
		PositionType: vcsutils.PointerOf("text"),
		NewLine:      &newLine,
		NewPath:      &newPath,
		OldLine:      &newLine,
		OldPath:      &oldPath,
	}

	// The GitLab REST API for creating a merge request discussion has strange behavior:
	// If the API call is not constructed precisely according to these rules, it may fail with an unclear error.
	// In all cases, 'new_path' and 'new_line' parameters are required.
	// - When commenting on a new file, do not include 'old_path' and 'old_line' parameters.
	// - When commenting on an existing file that has changed in the diff, omit 'old_path' and 'old_line' parameters.
	// - When commenting on an existing file that hasn't changed in the diff, include 'old_path' and 'old_line' parameters.

	client.logger.Debug(fmt.Sprintf("Create merge request discussion sent. newPath: %v newLine: %v oldPath: %v, oldLine: %v",
		newPath, newLine, oldPath, newLine))
	// Attempt to create a merge request discussion thread
	_, _, err := client.createMergeRequestDiscussion(ctx, projectID, comment.Content, pullRequestID, diffPosition)

	// Retry without oldLine and oldPath if the GitLab API call fails
	if err != nil {
		diffPosition.OldLine = nil
		diffPosition.OldPath = nil
		client.logger.Debug(fmt.Sprintf("Create merge request discussion second attempt sent. newPath: %v newLine: %v oldPath: %v, oldLine: %v",
			newPath, newLine, oldPath, newLine))
		_, _, err = client.createMergeRequestDiscussion(ctx, projectID, comment.Content, pullRequestID, diffPosition)
	}

	// If the comment creation still fails, return an error
	if err != nil {
		return fmt.Errorf("could not create a merge request discussion thread: %w", err)
	}

	return nil
}

func (client *GitLabClient) createMergeRequestDiscussion(ctx context.Context, projectID, content string, pullRequestID int, position *gitlab.PositionOptions) (*gitlab.Discussion, *gitlab.Response, error) {
	return client.glClient.Discussions.CreateMergeRequestDiscussion(projectID, pullRequestID, &gitlab.CreateMergeRequestDiscussionOptions{
		Body:     &content,
		Position: position,
	}, gitlab.WithContext(ctx))
}

// ListPullRequestReviewComments on GitLab
func (client *GitLabClient) ListPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	// Validate parameters
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "pullRequestID": strconv.Itoa(pullRequestID)}); err != nil {
		return nil, err
	}

	projectID := getProjectID(owner, repository)

	discussions, _, err := client.glClient.Discussions.ListMergeRequestDiscussions(projectID, pullRequestID, &gitlab.ListMergeRequestDiscussionsOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed fetching the list of merge requests discussions: %w", err)
	}

	var commentsInfo []CommentInfo
	for _, discussion := range discussions {
		commentsInfo = append(commentsInfo, mapGitLabNotesToCommentInfoList(discussion.Notes, discussion.ID)...)
	}

	return commentsInfo, nil
}

// ListPullRequestComments on GitLab
func (client *GitLabClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "pullRequestID": strconv.Itoa(pullRequestID)}); err != nil {
		return nil, err
	}
	commentsList, _, err := client.glClient.Notes.ListMergeRequestNotes(getProjectID(owner, repository), pullRequestID, &gitlab.ListMergeRequestNotesOptions{},
		gitlab.WithContext(ctx))
	if err != nil {
		return []CommentInfo{}, err
	}
	return mapGitLabNotesToCommentInfoList(commentsList, ""), nil
}

// DeletePullRequestReviewComment on GitLab
func (client *GitLabClient) DeletePullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...CommentInfo) error {
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "pullRequestID": strconv.Itoa(pullRequestID)}); err != nil {
		return err
	}
	for _, comment := range comments {
		var commentID int64
		if err := validateParametersNotBlank(map[string]string{"commentID": strconv.FormatInt(commentID, 10), "discussionID": comment.ThreadID}); err != nil {
			return err
		}
		if _, err := client.glClient.Discussions.DeleteMergeRequestDiscussionNote(getProjectID(owner, repository), pullRequestID, comment.ThreadID, int(commentID), gitlab.WithContext(ctx)); err != nil {
			return fmt.Errorf("an error occurred while deleting pull request review comment: %w", err)
		}
	}
	return nil
}

// DeletePullRequestComment on GitLab
func (client *GitLabClient) DeletePullRequestComment(ctx context.Context, owner, repository string, pullRequestID, commentID int) error {
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository}); err != nil {
		return err
	}
	if _, err := client.glClient.Notes.DeleteMergeRequestNote(getProjectID(owner, repository), pullRequestID, commentID, gitlab.WithContext(ctx)); err != nil {
		return fmt.Errorf("an error occurred while deleting pull request comment:\n%s", err.Error())
	}
	return nil
}

// GetLatestCommit on GitLab
func (client *GitLabClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	commits, err := client.GetCommits(ctx, owner, repository, branch)
	if err != nil {
		return CommitInfo{}, err
	}

	if len(commits) > 0 {
		return commits[0], nil
	}

	return CommitInfo{}, fmt.Errorf("no commits were returned for <%s/%s/%s>", owner, repository, branch)
}

// GetCommits on GitLab
func (client *GitLabClient) GetCommits(ctx context.Context, owner, repository, branch string) ([]CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"branch":     branch,
	})
	if err != nil {
		return nil, err
	}

	listOptions := &gitlab.ListCommitsOptions{
		RefName: &branch,
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: vcsutils.NumberOfCommitsToFetch,
		},
	}
	return client.getCommitsWithQueryOptions(ctx, owner, repository, listOptions)
}

func (client *GitLabClient) GetCommitsWithQueryOptions(ctx context.Context, owner, repository string, listOptions GitCommitsQueryOptions) ([]CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
	})
	if err != nil {
		return nil, err
	}

	return client.getCommitsWithQueryOptions(ctx, owner, repository, convertToListCommitsOptions(listOptions))
}

func convertToListCommitsOptions(options GitCommitsQueryOptions) *gitlab.ListCommitsOptions {
	t := time.Now()
	return &gitlab.ListCommitsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    options.Page,
			PerPage: options.PerPage,
		},
		Since: &options.Since,
		Until: &t,
	}
}

func (client *GitLabClient) getCommitsWithQueryOptions(ctx context.Context, owner, repository string, options *gitlab.ListCommitsOptions) ([]CommitInfo, error) {
	commits, _, err := client.glClient.Commits.ListCommits(getProjectID(owner, repository), options, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	var commitsInfo []CommitInfo
	for _, commit := range commits {
		commitInfo := mapGitLabCommitToCommitInfo(commit)
		commitsInfo = append(commitsInfo, commitInfo)
	}
	return commitsInfo, nil
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

	return RepositoryInfo{RepositoryVisibility: getGitLabProjectVisibility(project), CloneInfo: CloneInfo{HTTP: project.HTTPURLToRepo, SSH: project.SSHURLToRepo}}, nil
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

	commit, _, err := client.glClient.Commits.GetCommit(getProjectID(owner, repository), sha, nil, gitlab.WithContext(ctx))
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
func (client *GitLabClient) UnlabelPullRequest(ctx context.Context, owner, repository, label string, pullRequestID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return err
	}
	labels := gitlab.LabelOptions{label}
	_, _, err = client.glClient.MergeRequests.UpdateMergeRequest(getProjectID(owner, repository), pullRequestID, &gitlab.UpdateMergeRequestOptions{
		RemoveLabels: &labels,
	}, gitlab.WithContext(ctx))
	return err
}

// UploadCodeScanning on GitLab
func (client *GitLabClient) UploadCodeScanning(_ context.Context, _ string, _ string, _ string, _ string) (string, error) {
	return "", errGitLabCodeScanningNotSupported
}

// GetRepositoryEnvironmentInfo on GitLab
func (client *GitLabClient) GetRepositoryEnvironmentInfo(_ context.Context, _, _, _ string) (RepositoryEnvironmentInfo, error) {
	return RepositoryEnvironmentInfo{}, errGitLabGetRepoEnvironmentInfoNotSupported
}

// DownloadFileFromRepo on GitLab
func (client *GitLabClient) DownloadFileFromRepo(_ context.Context, owner, repository, branch, path string) ([]byte, int, error) {
	file, glResponse, err := client.glClient.RepositoryFiles.GetFile(getProjectID(owner, repository), path, &gitlab.GetFileOptions{Ref: &branch})
	var statusCode int
	if glResponse != nil && glResponse.Response != nil {
		statusCode = glResponse.Response.StatusCode
	}
	if err != nil {
		return nil, statusCode, err
	}
	if statusCode != http.StatusOK {
		return nil, statusCode, fmt.Errorf("expected %d status code while received %d status code", http.StatusOK, glResponse.StatusCode)
	}
	var content []byte
	if file != nil {
		content, err = base64.StdEncoding.DecodeString(file.Content)
	}
	return content, statusCode, err
}

func (client *GitLabClient) GetModifiedFiles(_ context.Context, owner, repository, refBefore, refAfter string) ([]string, error) {
	if err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"refBefore":  refBefore,
		"refAfter":   refAfter,
	}); err != nil {
		return nil, err
	}

	// No pagination is needed according to the official documentation at
	// https://docs.gitlab.com/ce/api/repositories.html#compare-branches-tags-or-commits
	compare, _, err := client.glClient.Repositories.Compare(
		getProjectID(owner, repository),
		&gitlab.CompareOptions{From: &refBefore, To: &refAfter},
	)
	if err != nil {
		return nil, err
	}

	fileNamesSet := datastructures.MakeSet[string]()
	for _, diff := range compare.Diffs {
		fileNamesSet.Add(diff.NewPath)
		fileNamesSet.Add(diff.OldPath)
	}
	_ = fileNamesSet.Remove("") // Make sure there are no blank filepath.
	fileNamesList := fileNamesSet.ToSlice()
	sort.Strings(fileNamesList)
	return fileNamesList, nil
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
		case vcsutils.TagPushed, vcsutils.TagRemoved:
			options.TagPushEvents = true
		}
	}
	return options
}

func getGitLabProjectVisibility(project *gitlab.Project) RepositoryVisibility {
	switch project.Visibility {
	case gitlab.PublicVisibility:
		return Public
	case gitlab.InternalVisibility:
		return Internal
	default:
		return Private
	}
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
		AuthorEmail:   commit.AuthorEmail,
	}
}

func mapGitLabNotesToCommentInfoList(notes []*gitlab.Note, discussionId string) (res []CommentInfo) {
	for _, note := range notes {
		res = append(res, CommentInfo{
			ID:       int64(note.ID),
			ThreadID: discussionId,
			Content:  note.Body,
			Created:  *note.CreatedAt,
		})
	}
	return
}

func (client *GitLabClient) mapGitLabMergeRequestToPullRequestInfoList(mergeRequests []*gitlab.MergeRequest, owner, repository string, withBody bool) (res []PullRequestInfo, err error) {
	for _, mergeRequest := range mergeRequests {
		var mergeRequestInfo PullRequestInfo
		if mergeRequestInfo, err = client.mapGitLabMergeRequestToPullRequestInfo(mergeRequest, withBody, owner, repository); err != nil {
			return
		}
		res = append(res, mergeRequestInfo)
	}
	return
}

func (client *GitLabClient) mapGitLabMergeRequestToPullRequestInfo(mergeRequest *gitlab.MergeRequest, withBody bool, owner, repository string) (PullRequestInfo, error) {
	var body string
	if withBody {
		body = mergeRequest.Description
	}
	sourceOwner := owner
	var err error
	if mergeRequest.SourceProjectID != mergeRequest.TargetProjectID {
		if sourceOwner, err = client.getProjectOwnerByID(mergeRequest.SourceProjectID); err != nil {
			return PullRequestInfo{}, err
		}
	}

	return PullRequestInfo{
		ID:     int64(mergeRequest.IID),
		Title:  mergeRequest.Title,
		Body:   body,
		Author: mergeRequest.Author.Username,
		Source: BranchInfo{
			Name:       mergeRequest.SourceBranch,
			Repository: repository,
			Owner:      sourceOwner,
		},
		URL: mergeRequest.WebURL,
		Target: BranchInfo{
			Name:       mergeRequest.TargetBranch,
			Repository: repository,
			Owner:      owner,
		},
	}, nil
}

func (client *GitLabClient) getProjectOwnerByID(projectID int) (string, error) {
	project, glResponse, err := client.glClient.Projects.GetProject(projectID, &gitlab.GetProjectOptions{})
	if err != nil {
		return "", err
	}
	if glResponse != nil {
		if err = vcsutils.CheckResponseStatusWithBody(glResponse.Response, http.StatusOK); err != nil {
			return "", err
		}
	}
	if project.Namespace == nil {
		return "", fmt.Errorf("could not fetch the name of the project owner. Project ID: %d", projectID)
	}
	return project.Namespace.Name, nil
}

func mapGitLabPullRequestState(state *vcsutils.PullRequestState) *string {
	var stateStringValue string
	switch *state {
	case vcsutils.Open:
		stateStringValue = "reopen"
	case vcsutils.Closed:
		stateStringValue = "close"
	default:
		return nil
	}
	return &stateStringValue
}
