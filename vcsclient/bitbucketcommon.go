package vcsclient

import (
	"errors"
)

var errLabelsNotSupported = errors.New("labels are not supported on Bitbucket")
var errCodeScanningNotSupported = errors.New("code Scanning is not supported on Bitbucket")

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
