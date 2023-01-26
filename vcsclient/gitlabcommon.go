package vcsclient

import (
	"errors"
)

var errGitLabCodeScanningNotSupported = errors.New("code scanning is not supported on Gitlab")
var errGitLabGetRepoEnvironmentInfoNotSupported = errors.New("get repository environment info is currently not supported on Bitbucket")
