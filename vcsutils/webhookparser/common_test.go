package webhookparser

import "os"

const (
	expectedRepoName     = "yahavi/hello-world"
	expectedBranch       = "main"
	expectedSourceBranch = "dev"
)

var token = []byte("abc123")

func closeReader(reader *os.File) {
	_ = reader.Close()
}
