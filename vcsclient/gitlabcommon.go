package vcsclient

import (
	"errors"
)

var errGitLabCodeScanningNotSupported = errors.New("code scanning is not supported on Gitlab")
var errGitLabGetRepoEnvironmentInfoNotSupported = errors.New("get repository environment info is currently not supported on Bitbucket")
var errGitLabListAppRepositories = errors.New("list app repositories is not supported on GitLab")

const (
	// https://docs.gitlab.com/ee/api/merge_requests.html#create-mr
	gitlabMergeRequestDetailsSizeLimit = 1048576
	// https://docs.gitlab.com/ee/api/notes.html#create-new-merge-request-note
	gitlabMergeRequestCommentSizeLimit = 1000000
)
