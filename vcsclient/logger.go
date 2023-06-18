package vcsclient

const (
	successfulRepoDownload   = "repository downloaded successfully. Starting with repository extraction..."
	successfulRepoExtraction = "Extracted repository successfully"
	creatingPullRequest      = "Creating new pull request:"
	fetchingOpenPullRequests = "Fetching open pull requests in"
	fetchingPullRequestById  = "Fetching pull requests by id in "
	uploadingCodeScanning    = "Uploading code scanning for:"
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
