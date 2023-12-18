package vcsutils

const (
	SuccessfulRepoDownload   = "repository downloaded successfully. Starting with repository extraction..."
	SuccessfulRepoExtraction = "Extracted repository successfully"
	CreatingPullRequest      = "Creating new pull request:"

	UpdatingPullRequest      = "Updating details of pull request ID:"
	FetchingOpenPullRequests = "Fetching open pull requests in"
	FetchingPullRequestById  = "Fetching pull requests by id in"
	UploadingCodeScanning    = "Uploading code scanning for:"

	FailedForkedRepositoryExtraction = "Failed to extract forked repository owner"
)

type Log interface {
	Debug(a ...interface{})
	Info(a ...interface{})
	Warn(a ...interface{})
	Error(a ...interface{})
	Output(a ...interface{})
}

type EmptyLogger struct{}

func (el EmptyLogger) Debug(_ ...interface{}) {
}

func (el EmptyLogger) Info(_ ...interface{}) {
}

func (el EmptyLogger) Warn(_ ...interface{}) {
}

func (el EmptyLogger) Error(_ ...interface{}) {
}

func (el EmptyLogger) Output(_ ...interface{}) {
}
