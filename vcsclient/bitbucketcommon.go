package vcsclient

import "fmt"

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

func getLabelsUnsupportedError() error {
	return fmt.Errorf("labels are not supported on Bitbucket")
}
