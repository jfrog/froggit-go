<div align="center">

# Froggit-Go

[![Frogbot](images/header.png)](#readme)

</div>

Froggit-Go is a Go library, allowing to perform actions on VCS providers.
Currently supported providers are: [GitHub](#github), [Bitbucket Server](#bitbucket-server)
, [Bitbucket Cloud](#bitbucket-cloud), [Azure Repos](#azure-repos) and [GitLab](#gitlab).

## Project status

[![Scanned by Frogbot](https://raw.github.com/jfrog/frogbot/master/images/frogbot-badge.svg)](https://github.com/jfrog/frogbot#readme)
[![Test](https://github.com/jfrog/froggit-go/actions/workflows/test.yml/badge.svg)](https://github.com/jfrog/froggit-go/actions/workflows/test.yml)
[![Coverage Status](https://coveralls.io/repos/github/jfrog/froggit-go/badge.svg?branch=master)](https://coveralls.io/github/jfrog/froggit-go?branch=master)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/jfrog/froggit-go)](https://goreportcard.com/report/github.com/jfrog/froggit-go)

## Usage

- [Froggit-Go](#froggit-go)
  - [Project status](#project-status)
  - [Usage](#usage)
    - [VCS Clients](#vcs-clients)
      - [Create Clients](#create-clients)
        - [GitHub](#github)
        - [GitLab](#gitlab)
        - [Bitbucket Server](#bitbucket-server)
        - [Bitbucket Cloud](#bitbucket-cloud)
        - [Azure Repos](#azure-repos)
      - [Test Connection](#test-connection)
      - [List Repositories](#list-repositories)
      - [List App Repositories](#list-app-repositories)
      - [List Branches](#list-branches)
      - [List Pull Request Reviews](#list-pull-request-reviews)
      - [Download Repository](#download-repository)
      - [Create Webhook](#create-webhook)
      - [Update Webhook](#update-webhook)
      - [Delete Webhook](#delete-webhook)
      - [Set Commit Status](#set-commit-status)
      - [Get Commit Status](#get-commit-status)
      - [Create Pull Request](#create-pull-request)
      - [Update Pull Request](#update-pull-request)
      - [Get Pull Request By ID](#get-pull-request-by-id)
      - [List Open Pull Requests](#list-open-pull-requests)
      - [List Open Pull Requests With Body](#list-open-pull-requests-with-body)
      - [Add Pull Request Comment](#add-pull-request-comment)
      - [Add Pull Request Review Comments](#add-pull-request-review-comments)
      - [List Pull Request Comments](#list-pull-request-comments)
      - [List Pull Request Review Comments](#list-pull-request-review-comments)
      - [Delete Pull Request Comment](#delete-pull-request-comment)
      - [Delete Pull Request Review Comments](#delete-pull-request-review-comments)
      - [Get Commits](#get-commits)
      - [Get Commits With Options](#get-commits-with-options)
      - [Get Latest Commit](#get-latest-commit)
      - [Get Commit By SHA](#get-commit-by-sha)
      - [List Pull Requests associated with a Commit](#list-pull-requests-associated-with-a-commit)
      - [Get List of Modified Files](#get-list-of-modified-files)
      - [Add Public SSH Key](#add-public-ssh-key)
      - [Get Repository Info](#get-repository-info)
      - [Get Repository Environment Info](#get-repository-environment-info)
      - [Create a label](#create-a-label)
      - [Get a label](#get-a-label)
      - [List Pull Request Labels](#list-pull-request-labels)
      - [Unlabel Pull Request](#unlabel-pull-request)
      - [Upload Code Scanning](#upload-code-scanning)
      - [Download a File From a Repository](#download-a-file-from-a-repository)
      - [Create a branch](#create-branch)
      - [Allow workflows on organization](#allow-workflows)
      - [Add Organization Secret](#add-organization-secret)
      - [Get Repo Collaborators](#get-repo-collaborators)
      - [Get Repo Teams By Permissions](#get-repo-teams-by-permissions)
      - [Create Or Update Environment](#create-or-update-environment)
      - [CommitAndPushFiles](#commit-and-push-files)
      - [Merge Pull Request](#merge-pull-request)
      - [Create Pull Request Detailed](#create-pull-request-detailed)
      - [Upload Snapshot To Dependency Graph](#upload-snapshot-to-dependency-graph)
    - [Webhook Parser](#webhook-parser)

### VCS Clients

#### Create Clients

##### GitHub

GitHub api v3 is used

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsutils.GitHub
// API endpoint to GitHub. Leave empty to use the default - https://api.github.com
apiEndpoint := "https://github.example.com"
// Access token to GitHub
token := "secret-github-token"
// Logger
// [Optional]
// Supported logger is a logger that implements the Log interface.
// More information - https://github.com/jfrog/froggit-go/blob/master/vcsclient/logger.go
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Token(token).Build()
```

##### GitLab

GitLab api v4 is used.

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsutils.GitLab
// API endpoint to GitLab. Leave empty to use the default - https://gitlab.com
apiEndpoint := "https://gitlab.example.com"
// Access token to GitLab
token := "secret-gitlab-token"
// Logger
// [Optional]
// Supported logger is a logger that implements the Log interface.
// More information - https://github.com/jfrog/froggit-go/blob/master/vcsclient/logger.go
logger := logger

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Token(token).Build()
```

##### Bitbucket Server

Bitbucket api 1.0 is used.

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsclient.BitbucketServer
// API endpoint to Bitbucket server. Typically ends with /rest.
apiEndpoint := "https://git.acme.com/rest"
// Access token to Bitbucket
token := "secret-bitbucket-token"
// Logger
// [Optional]
// Supported logger is a logger that implements the Log interface.
// More information - https://github.com/jfrog/froggit-go/blob/master/vcsclient/logger.go
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Token(token).Build()
```

##### Bitbucket Cloud

Bitbucket cloud api version 2.0 is used and the version should be added to the apiEndpoint.

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsutils.BitbucketCloud
// API endpoint to Bitbucket cloud. Leave empty to use the default - https://api.bitbucket.org/2.0
apiEndpoint := "https://bitbucket.example.com"
// Bitbucket username
username := "bitbucket-user"
// Password or Bitbucket "App Password'
token := "secret-bitbucket-token"
// Logger
// [Optional]
// Supported logger is a logger that implements the Log interface.
// More information - https://github.com/jfrog/froggit-go/blob/master/vcsclient/logger.go
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Username(username).Token(token).Build()
```

##### Azure Repos

Azure DevOps api version v6 is used.

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsutils.AzureRepos
// API endpoint to Azure Repos. Set the organization.
apiEndpoint := "https://dev.azure.com/<organization>"
// Personal Access Token to Azure DevOps
token := "secret-azure-devops-token"
// Logger
// [Optional]
// Supported logger is a logger that implements the Log interface.
// More information - https://github.com/jfrog/froggit-go/blob/master/vcsclient/logger.go
logger := log.Default()
// Project name
project := "name-of-the-relevant-project"

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Token(token).Project(project).Build()
```

#### Test Connection

```go
// Go context
ctx := context.Background()

err := client.TestConnection(ctx)
```

#### List Repositories

```go
// Go context
ctx := context.Background()

repositories, err := client.ListRepositories(ctx)
```

#### List App Repositories

Returns a map between all accessible Apps and their list of repositories.  
Note: Currently supported for GitHub Apps only.

```go
// Go context
ctx := context.Background()

// List all repositories accessible by the app (for example, a GitHub App installation)
appRepositories, err := client.ListAppRepositories(ctx)
if err != nil {
    // handle error
}
for owner, repos := range appRepositories {
    fmt.Printf("Owner: %s\n", owner)
    for _, repo := range repos {
        fmt.Printf("  - %s\n", repo)
    }
}
```

#### List Branches

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"

repositoryBranches, err := client.ListBranches(ctx, owner, repository)
```

#### List Pull Request Reviews

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestID := 1

// List all reviews for pull request 1
reviews, err := client.ListPullRequestReviews(ctx, owner, repository, pullRequestID)
```

#### Download Repository

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Repository branch
branch := "master"
// Local path in the file system
localPath := "/Users/frogger/code/jfrog-cli"

repositoryBranches, err := client.DownloadRepository(ctx, owner, repository, branch, localPath)
```

#### Create Webhook

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// The event to watch
webhookEvent := vcsutils.Push
// VCS repository
repository := "jfrog-cli"
// Optional - Webhooks on branches are supported only on GitLab
branch := ""
// The URL to send the payload upon a webhook event
payloadURL := "https://acme.jfrog.io/integration/api/v1/webhook/event"

// token - A token used to validate identity of the incoming webhook.
// In GitHub and Bitbucket server the token verifies the sha256 signature of the payload.
// In GitLab and Bitbucket cloud the token compared to the token received in the incoming payload.
id, token, err := client.CreateWebhook(ctx, owner, repository, branch, "https://jfrog.com", webhookEvent)
```

#### Update Webhook

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Optional - Webhooks on branches are supported only on GitLab
branch := ""
// The URL to send the payload upon a webhook event
payloadURL := "https://acme.jfrog.io/integration/api/v1/webhook/event"
// A token to validate identity of the webhook, created by CreateWebhook command
token := "abc123"
// The webhook ID returned by the CreateWebhook API, which created this webhook
webhookID := "123"
// The event to watch
webhookEvent := vcsutils.PrCreated

err := client.UpdateWebhook(ctx, owner, repository, branch, "https://jfrog.com", token, webhookID, webhookEvent)
```

#### Delete Webhook

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// The webhook ID returned by the CreateWebhook API, which created this webhook
webhookID := "123"

err := client.DeleteWebhook(ctx, owner, repository, webhookID)
```

#### Set Commit Status

```go
// Go context
ctx := context.Background()
// One of Pass, Fail, Error, or InProgress
commitStatus := vcsclient.Pass
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Branch or commit or tag on GitHub and GitLab, commit on Bitbucket
ref := "5c05522fecf8d93a11752ff255c99fcb0f0557cd"
// Title of the commit status
title := "Xray scanning"
// Description of the commit status
description := "Run JFrog Xray scan"
// URL leads to the platform to provide more information, such as Xray scanning results
detailsURL := "https://acme.jfrog.io/ui/xray-scan-results-url"

err := client.SetCommitStatus(ctx, commitStatus, owner, repository, ref, title, description, detailsURL)
```

#### Get Commit Status

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Commit tag on GitHub and GitLab, commit on Bitbucket
ref := "5c05522fecf8d93a11752ff255c99fcb0f0557cd"

commitStatuses, err := client.GetCommitStatus(ctx, owner, repository, ref)
```

##### Create Pull Request

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Source pull request branch
sourceBranch := "dev"
// Target pull request branch
targetBranch := "main"
// Pull request title
title := "Pull request title"
// Pull request description
description := "Pull request description"

err := client.CreatePullRequest(ctx, owner, repository, sourceBranch, targetBranch, title, description)
```

##### Update Pull Request

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Target pull request branch, leave empty for no change.
targetBranch := "main"
// Pull request title
title := "Pull request title"
// Pull request description
body := "Pull request description"
// Pull request ID
id := "1"
// Pull request state
state := vcsutils.Open

err := client.UpdatePullRequest(ctx, owner, repository, title, body, targetBranch, id, state)
```

#### List Open Pull Requests With Body

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"

openPullRequests, err := client.ListOpenPullRequestsWithBody(ctx, owner, repository)
```

#### List Open Pull Requests

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"

openPullRequests, err := client.ListOpenPullRequests(ctx, owner, repository)
```

#### Get Pull Request By ID

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestId := 1

openPullRequests, err := client.GetPullRequestByID(ctx, owner, repository, pullRequestId)
```

##### Add Pull Request Comment

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Comment content
content := "Comment content"
// Pull Request ID
pullRequestID := 5

err := client.AddPullRequestComment(ctx, owner, repository, content, pullRequestID)
```

##### Add Pull Request Review Comments

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestID := 5
// Pull Request Comment
comments := []PullRequestComment{
  {
    CommentInfo: CommentInfo{
      Content: "content",
    },
    PullRequestDiff: PullRequestDiff{
      OriginalFilePath: index.js   
      OriginalStartLine: 1
      OriginalEndLine: 1
      OriginalStartColumn: 1
      OriginalEndColumn: 1  
      NewFilePath: index.js        
      NewStartLine: 1       
      NewEndLine: 1         
      NewStartColumn: 1     
      NewEndColumn: 1       
    },
  }
}


err := client.AddPullRequestReviewComments(ctx, owner, repository, pullRequestID, comments...)
```

##### List Pull Request Comments

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestID := 5

pullRequestComments, err := client.ListPullRequestComment(ctx, owner, repository, pullRequestID)
```

##### List Pull Request Review Comments

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestID := 5

pullRequestComments, err := client.ListPullRequestReviewComments(ctx, owner, repository, pullRequestID)
```

##### Delete Pull Request Comment

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestID := 5
// Comment ID
commentID := 17

err := client.DeletePullRequestComment(ctx, owner, repository, pullRequestID, commentID)
```

##### Delete Pull Request Review Comments

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestID := 5
// Comment ID
comments := []CommentInfo{
  {
    ID: 2
    // For GitLab
    ThreadID: 7
  }
}

err := client.DeletePullRequestComment(ctx, owner, repository, pullRequestID, comments...)
```


#### Get Commits

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// VCS branch
branch := "dev"

// Commits information of the latest branch commits 
commitInfo, err := client.GetCommits(ctx, owner, repository, branch)
```

#### Get Commits With Options

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"

// Commits query options 
options := GitCommitsQueryOptions{
  Since: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
  Until: time.Now(),
  ListOptions: ListOptions{
	  Page:    1,
	  PerPage: 30,
    },
  }

result, err := client.GetCommitsWithQueryOptions(ctx, owner, repository, options)
```

#### Get Latest Commit

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// VCS branch
branch := "dev"

// Commit information of the latest commit
commitInfo, err := client.GetLatestCommit(ctx, owner, repository, branch)
```

#### Get Commit By SHA

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// SHA-1 hash of the commit
sha := "abcdef0123abcdef4567abcdef8987abcdef6543"

// Commit information of requested commit
commitInfo, err := client.GetCommitBySha(ctx, owner, repository, sha)
```

### 
```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Commit SHA
commitSHA := "abcdef0123abcdef4567abcdef8987abcdef6543"

// List pull requests associated with a specific commit
pullRequests, err := client.ListPullRequestsAssociatedWithCommit(ctx, owner, repository, commitSHA)
```

#### Get List of Modified Files

The `refBefore...refAfter` syntax is used.
More about it can be found at [Commit Ranges](https://git-scm.com/book/en/v2/Git-Tools-Revision-Selection) Git
documentation.

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// SHA-1 hash of the commit or tag or a branch name
refBefore := "abcdef0123abcdef4567abcdef8987abcdef6543"
// SHA-1 hash of the commit or tag or a branch name
refAfter := "main"

filePaths, err := client.GetModifiedFiles(ctx, owner, repository, refBefore, refAfter)
```

#### Add Public SSH Key

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// An identifier for the key
keyName := "my ssh key"
// The public SSH key
publicKey := "ssh-rsa AAAA..."
// Access permission of the key: vcsclient.Read or vcsclient.ReadWrite
permission = vcsclient.Read

// Add a public SSH key to a repository
err := client.AddSshKeyToRepository(ctx, owner, repository, keyName, publicKey, permission)
```

#### Get Repository Info

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"

// Get information about repository
repoInfo, err := client.GetRepositoryInfo(ctx, owner, repository)
```

#### Get Repository Environment Info

Notice - Get Repository Environment Info is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Environment name
name := "frogbot"

// Get information about repository environment
repoEnvInfo, err := client.GetRepositoryEnvironmentInfo(ctx, owner, repository, name)
```

#### Create a label

Notice - Labels are not supported in Bitbucket

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Label info
labelInfo := LabelInfo{
  Name:        "label-name",
  Description: "label description",
  Color:       "4AB548",
}
// Create a label
err := client.CreateLabel(ctx, owner, repository, labelInfo)
```

#### Get a label

Notice - Labels are not supported in Bitbucket

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Label name
labelName := "label-name"

// Get a label named "label-name"
labelInfo, err := client.GetLabel(ctx, owner, repository, labelName)
```

#### List Pull Request Labels

Notice - Labels are not supported in Bitbucket

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Pull Request ID
pullRequestID := 5

// List all labels assigned to pull request 5
pullRequestLabels, err := client.ListPullRequestLabels(ctx, owner, repository, pullRequestID)
```

#### Unlabel Pull Request

Notice - Labels are not supported in Bitbucket

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Label name
name := "label-name"
// Pull Request ID
pullRequestID := 5

// Remove label "label-name" from pull request 5
err := client.UnlabelPullRequest(ctx, owner, repository, name, pullRequestID)
```

#### Upload Code Scanning

Notice - Code Scanning is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// The account owner of the git repository
owner := "user"
// The name of the repository
repo := "my_repo"
// The branch name for which the code scanning is relevant
branch := "my_branch"
// A string representing the code scanning results
scanResults := "results"

// Uploads the scanning analysis file to the relevant git provider
sarifID, err := client.UploadCodeScanning(ctx, owner, repo, branch, scanResults)
```

#### Download a File From a Repository

Note - This API is currently not supported for Bitbucket Cloud.

```go
// Go context
ctx := context.Background()
// The account owner of the git repository
owner := "user"
// The name of the repository
repo := "my_repo"
// The branch name for which the code scanning is relevant
branch := "my_branch"
// A string representing the file path in the repository
path := "path"

// Downloads a file from a repository
content, statusCode, err := client.DownloadFileFromRepo(ctx, owner, repo, branch, path)
```


#### Create Branch

Notice - Create Branch is currently supported on GitHub only.

```go
// Go Context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VSC Repository
repository := "jfrog-cli"
// Source branch to create a new branch from
sourceBranch := "main"
// New branch name
newBranch := "my-new-branch"

// Create a new branch
err = client.CreateBranch(ctx, owner, repository, sourceBranch, newBranch)
```

#### Allow Workflows

Notice - Allow Workflows is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization 
owner := "jfrog"

// Allow workflows for the organization
err = client.AllowWorkflows(ctx, owner)
```

#### Add Organization Secret

Notice - Add Organization Secrets currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization
owner := "jfrog"
// Secret name
secret := "key"
// Secret value, will be encrypted by froggit
secretValue := "some-secret-value"

// Add a secret to the organization
err = client.AddOrganizationSecret(ctx, owner, secret, secretValue)
```

#### Create Organization Variable

Notice - Create Organization Variable is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization
owner := "jfrog"
// Variable name
variableName := "JF_URL"
// Variable value
variableValue := "https://acme.jfrog.io/"

// Add a variable to the organization
err = client.CreateOrgVar(ctx, owner, variableName, variableValue)

#### Get Repo Collaborators

Notice - Get Repo Collaborators is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization
owner := "jfrog"
// Repository name
repo := "jfrog-cli"
// Affiliation type, can be one of the following: all, direct, outside, member
affiliation := "direct"
// Permission type, can be one of the following: read, write, admin, maintain, triage
permission := "maintain"

// Get the list of collaborators for a specific repository
collaborators, err := client.GetRepoCollaborators(ctx, owner, repo, affiliation, permission)
```

#### Get Repo Teams By Permissions

Notice - Get Repo Teams By Permissions currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization
owner := "jfrog"
// Repository name
repo := "jfrog-cli"
// Permission type, can be one of the following: read, write, admin, maintain, triage
permissions := []string{"maintain", "admin"}

// Get the list of teams with specific permissions for a repository
teams, err := client.GetRepoTeamsByPermissions(ctx, owner, repo, permissions)
```

#### Create Or Update Environment

Notice - Create Or Update Environment is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization
owner := "jfrog-org"
// Repository name
repo := "big-npm"
// Repository environment name
envName := "frogbot"
// List of teams ids to add to the environment
teams := []int64{12345678}
// List of user names to add to the environment
users := []string{"eyalk007"}
	
// Create or update the environment
err = client.CreateOrUpdateEnvironment(ctx, owner, repo, envName, teams, users)
```

#### Commit And Push Files

Notice - Commit And Push Files is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization
owner := "jfrog"
// Repository name
repo := "jfrog-cli"
// Source branch name
sourceBranch := "feature-branch"
// Commit message
commitMessage := "example commit message" 
// Author name
author := "example"
//Files To commit
filesToCommit := []vcsclient.FileToCommit{{
		Path:    ".github/workflows/example.yml",
		Content: "hello world",
	}}
//Author email
authorEmail := "example@gmail.com"

// Commit and push files to the repository in the source branch
err = client.CommitAndPushFiles(ctx, owner, repo, sourceBranch, commitMessage, author, authorEmail, filesToCommit)
```

#### Merge Pull Request

Notice - Merge Pull Request is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization
owner := "jfrog"
// Repository name
repo := "jfrog-cli"
// pull request number
prNumber := 134
// Commit message, empty will use the default commit message
commitMessage := "example commit message"

// Merge the pull request
err = client.MergePullRequest(ctx, owner, repo, prNumber, commitMessage)
```

#### Create Pull Request Detailed

Notice - Create Pull Request Detailed is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Source pull request branch
sourceBranch := "dev"
// Target pull request branch
targetBranch := "main"
// Pull request title
title := "Pull request title"
// Pull request description
description := "Pull request description"

// Creates a pull request and returns its number and URL.
prInfo,err := client.CreatePullRequestDetailed(ctx, owner, repository, sourceBranch, targetBranch, title, description)
```

#### Upload Snapshot To Dependency Graph

Notice - Upload Snapshot To Dependency Graph is currently supported on GitHub only.

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// SBOM snapshot containing dependency information
snapshot := &vcsclient.SbomSnapshot{
    Version: 0,
    Sha:     "5c05522fecf8d93a11752ff255c99fcb0f0557cd",
    Ref:     "refs/heads/main",
    Job: &vcsclient.JobInfo{
        Correlator: "job-correlator",
        ID:         "job-id",
    },
    Detector: &vcsclient.DetectorInfo{
        Name:    "detector-name",
        Version: "1.0.0",
        Url:     "https://example.com/detector",
    },
    Scanned: time.Now(),
    Manifests: map[string]*vcsclient.Manifest{
        "package.json": {
            Name: "package.json",
            File: &vcsclient.FileInfo{
                SourceLocation: "package.json",
            },
            Resolved: map[string]*vcsclient.ResolvedDependency{
                "lodash": {
                    PackageURL:   "pkg:npm/lodash@4.17.21",
                    Dependencies: []string{},
                },
            },
        },
    },
}

// Uploads the SBOM snapshot to the GitHub dependency graph tab
err := client.UploadSnapshotToDependencyGraph(ctx, owner, repository, snapshot)
```

### Webhook Parser

```go
// Go context
ctx := context.Background()
// Logger
logger := vcsclient.EmptyLogger{}
// Webhook contextual information
origin := webhookparser.WebhookOrigin{
  // The VCS provider (required)
  VcsProvider: vcsutils.GitHub,
  // Optional URL of the VCS provider (used for building some URLs)
  OriginURL: "https://api.github.com",
  // Token to authenticate incoming webhooks. If empty, signature will not be verified. 
  // The token is a random key generated in the CreateWebhook command. 
  Token: []byte("abc123"),
}
// The HTTP request of the incoming webhook
request := http.Request{}

webhookInfo, err := webhookparser.ParseIncomingWebhook(ctx, logger, origin, request)
```
