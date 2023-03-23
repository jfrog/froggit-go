package vcsclient

import (
	"errors"
)

var errLabelsNotSupported = errors.New("labels are not supported on Bitbucket")
var errBitbucketCodeScanningNotSupported = errors.New("code scanning is not supported on Bitbucket")

var errBitbucketDownloadFileFromRepoNotSupported = errors.New("download file from repo is currently not supported on Bitbucket")
var errBitbucketGetRepoEnvironmentInfoNotSupported = errors.New("get repository environment info is currently not supported on Bitbucket")

func getBitbucketCommitState(commitState CommitStatus) string {
	switch commitState {
	case Pass:
		return "SUCCESSFUL"
	case Fail, Error:
		return "FAILED"
	case InProgress:
		return "INPROGRESS"
	}
	return ""
}
