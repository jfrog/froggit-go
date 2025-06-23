package vcsclient

import (
	"errors"
)

var errGitLabCodeScanningNotSupported = errors.New("code scanning is not supported on Gitlab")
var errGitLabGetRepoEnvironmentInfoNotSupported = errors.New("get repository environment info is currently not supported on Gitlab")
var errGitLabCreateBranchNotSupported = errors.New("creating a branch is not supported on Gitlab")
var errGitLabAllowWorkflowsNotSupported = errors.New("allow workflows is not supported on Gitlab")
var errGitLLabAddOrganizationSecretNotSupported = errors.New("adding organization secret is not supported on Gitlab")
var errGitLabCommitAndPushFilesNotSupported = errors.New("commit and push files is not supported on Gitlab")
var errGitLabGetCollaboratorsNotSupported = errors.New("get collaborators is not supported on Gitlab")
var errGitLabGetRepoTeamsByPermissionsNotSupported = errors.New("get repository Teams By permissions is not supported on Gitlab")
var errGitLabCreateOrUpdateEnvironmentNotSupported = errors.New("create or update environment is not supported on Gitlab")
var errGitLabMergePullRequestNotSupported = errors.New("merging pull request is not supported on Gitlab")
var errGitLabListAppRepositories = errors.New("list app repositories is not supported on GitLab")
var errGitlabCreatePullRequestDetailedNotSupported = errors.New("creating pull request detailed is not supported on Gitlab")

const (
	// https://docs.gitlab.com/ee/api/merge_requests.html#create-mr
	gitlabMergeRequestDetailsSizeLimit = 1048576
	// https://docs.gitlab.com/ee/api/notes.html#create-new-merge-request-note
	gitlabMergeRequestCommentSizeLimit = 1000000
)
