package vcsclient

import (
	"bytes"
	"context"
	"log"
	"strconv"

	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/xanzy/go-gitlab"
)

type namespaceKind string

const (
	user  namespaceKind = "user"
	group               = "group"
)

type GitLabClient struct {
	glClient *gitlab.Client
	context  context.Context
	logger   *log.Logger
}

func NewGitLabClient(context context.Context, logger *log.Logger, vcsInfo *VcsInfo) (*GitLabClient, error) {
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
		context:  context,
		logger:   logger,
	}, nil
}

func (client *GitLabClient) TestConnection() error {
	_, _, err := client.glClient.Projects.ListProjects(nil, gitlab.WithContext(client.context))
	return err
}

func (client *GitLabClient) ListRepositories() (map[string][]string, error) {
	results := make(map[string][]string)
	groups, _, err := client.glClient.Groups.ListGroups(nil, gitlab.WithContext(client.context))
	if err != nil {
		return results, err
	}
	for _, group := range groups {
		for pageId := 1; ; pageId++ {
			options := &gitlab.ListGroupProjectsOptions{ListOptions: gitlab.ListOptions{Page: pageId}}
			projects, response, err := client.glClient.Groups.ListGroupProjects(group.Path, options, gitlab.WithContext(client.context))
			if err != nil {
				return nil, err
			}

			for _, project := range projects {
				results[group.Path] = append(results[group.Path], project.Path)
			}
			if pageId >= response.TotalPages {
				break
			}
		}

	}
	return results, nil
}

func (client *GitLabClient) ListBranches(owner, repository string) ([]string, error) {
	branches, _, err := client.glClient.Branches.ListBranches(client.getProjectId(owner, repository), nil, gitlab.WithContext(client.context))
	if err != nil {
		return []string{}, err
	}

	results := []string{}
	for _, branch := range branches {
		results = append(results, branch.Name)
	}
	return results, nil
}

func (client *GitLabClient) CreateWebhook(owner, repository, branch, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	token := vcsutils.CreateToken()
	projectHook := createProjectHook(branch, payloadUrl, webhookEvents...)
	options := &gitlab.AddProjectHookOptions{
		Token:                  &token,
		URL:                    &projectHook.URL,
		MergeRequestsEvents:    &projectHook.MergeRequestsEvents,
		PushEvents:             &projectHook.PushEvents,
		PushEventsBranchFilter: &projectHook.PushEventsBranchFilter,
	}
	response, _, err := client.glClient.Projects.AddProjectHook(client.getProjectId(owner, repository), options, gitlab.WithContext(client.context))
	if err != nil {
		return "", "", err
	}
	return strconv.Itoa(response.ID), token, nil
}

func (client *GitLabClient) UpdateWebhook(owner, repository, branch, payloadUrl, token, webhookId string, webhookEvents ...vcsutils.WebhookEvent) error {
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
	_, _, err = client.glClient.Projects.EditProjectHook(client.getProjectId(owner, repository), intWebhook, options, gitlab.WithContext(client.context))
	return err
}

func (client *GitLabClient) DeleteWebhook(owner, repository, webhookId string) error {
	intWebhook, err := strconv.Atoi(webhookId)
	if err != nil {
		return err
	}
	_, err = client.glClient.Projects.DeleteProjectHook(client.getProjectId(owner, repository), intWebhook, gitlab.WithContext(client.context))
	return err
}

func (client *GitLabClient) SetCommitStatus(commitStatus CommitStatus, owner, repository, ref, title, description, detailsUrl string) error {
	options := &gitlab.SetCommitStatusOptions{
		State:       gitlab.BuildStateValue(getGitLabCommitState(commitStatus)),
		Ref:         &ref,
		Name:        &title,
		Description: &description,
		TargetURL:   &detailsUrl,
	}
	_, _, err := client.glClient.Commits.SetCommitStatus(client.getProjectId(owner, repository), ref, options, gitlab.WithContext(client.context))
	return err
}

func (client *GitLabClient) DownloadRepository(owner, repository, branch, localPath string) error {
	format := "tar.gz"
	options := &gitlab.ArchiveOptions{
		Format: &format,
		SHA:    &branch,
	}
	response, _, err := client.glClient.Repositories.Archive(client.getProjectId(owner, repository), options, gitlab.WithContext(client.context))
	if err != nil {
		return err
	}
	return vcsutils.Untar(localPath, bytes.NewReader(response), true)
}

func (client *GitLabClient) Push(owner, repository string, branch string) error {
	return nil
}

func (client *GitLabClient) CreatePullRequest(owner, repository, sourceBranch, targetBranch, title, description string) error {
	options := &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		Description:  &description,
		SourceBranch: &sourceBranch,
		TargetBranch: &targetBranch,
	}
	_, _, err := client.glClient.MergeRequests.CreateMergeRequest(client.getProjectId(owner, repository), options, gitlab.WithContext(client.context))
	return err
}

func (client *GitLabClient) getProjectId(owner, project string) string {
	return owner + "/" + project
}

func createProjectHook(branch string, payloadUrl string, webhookEvents ...vcsutils.WebhookEvent) *gitlab.ProjectHook {
	options := &gitlab.ProjectHook{URL: payloadUrl}
	for _, webhookEvent := range webhookEvents {
		switch webhookEvent {
		case vcsutils.PrCreated, vcsutils.PrEdited:
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
