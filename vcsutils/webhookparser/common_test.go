package webhookparser

import (
	"io"
)

const (
	expectedOwner        = "yahavi"
	expectedRepoName     = "hello-world"
	expectedBranch       = "main"
	expectedSourceBranch = "dev"
)

var token = []byte("abc123")

func close(closer io.Closer) {
	_ = closer.Close()
}
