package vcsclient

import (
	"errors"
)

var errGitLabCodeScanningNotSupported = errors.New("code scanning is not supported on Gitlab")
