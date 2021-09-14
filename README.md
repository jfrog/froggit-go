# Froggit Go

Froggit-Go is a set of Go tools allowing to perform actions on VCS providers.
Currently supported providers are: [GitHub](#github), [Bitbucket Server](#bitbucket-server), [Bitbucket Cloud](#bitbucket-cloud), and [GitLab](#gitlab).

## Project status

[![Test](https://github.com/jfrog/froggit-go/actions/workflows/test.yml/badge.svg)](https://github.com/jfrog/froggit-go/actions/workflows/test.yml)

## Usage

- [VCS Clients](#vcs-clients)
  - [Create Clients](#create-clients)
    - [GitHub](#github)
    - [GitLab](#gitlab)
    - [Bitbucket Server](#bitbucket-server)
    - [Bitbucket Cloud](#bitbucket-cloud)
  - [Test Connection](#test-connection)
  - [List Repositories](#list-repositories)
  - [List Branches](#list-branches)
  - [Download Repository](#download-repository)
  - [Create Webhook](#create-webhook)
  - [Update Webhook](#update-webhook)
  - [Delete Webhook](#delete-webhook)
  - [Set Commit Status](#set-commit-status)
  - [Create Pull Request](#create-pull-request)
- [Webhook Parser](#webhook-parser)

### VCS Clients

#### Create Clients

##### GitHub

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsutils.GitHub
// API endpoint to GitHub. Leave empty to use the default - https://api.github.com
apiEndpoint := "https://github.example.com"
// Access token to GitHub
token := "secret-github-token"
// Optional Parameters:
// The context
context := context.Background()
// Logger
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Token(token).Build()
```

##### GitLab

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsutils.GitLab
// API endpoint to GitLab. Leave empty to use the default - https://gitlab.com
apiEndpoint := "https://gitlab.example.com"
// Access token to GitLab
token := "secret-gitlab-token"
// Optional Parameters:
// The context
context := context.Background()
// Logger
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Token(token).Build()
```

##### Bitbucket Server

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsclient.BitbucketServer
// API endpoint to Bitbucket server. Typically ends with /rest.
apiEndpoint := "https://git.acme.com/rest"
// Access token to Bitbucket
token := "secret-bitbucket-token"
// Optional Parameters:
// The context
context := context.Background()
// Logger
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Token(token).Build()
```

##### Bitbucket Cloud

```go
// The VCS provider. Cannot be changed.
vcsProvider := vcsutils.BitbucketCloud
// API endpoint to Bitbucket cloud. Leave empty to use the default - https://api.bitbucket.org/2.0
apiEndpoint := "https://bitbucket.example.com"
// Bitbucket username
username := "bitbucket-user"
// Password or Bitbucket "App Password'
token := "secret-bitbucket-token"
// Optional Parameters:
// The context
context := context.Background()
// Logger
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Username(username).Token(token).Build()
```

#### Test Connection

```go
err := client.TestConnection()
```

#### List Repositories

```go
repositories, err := client.ListRepositories()
```

#### List Branches

```go
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"

repositoryBranches, err := client.ListBranches(owner, repository)
```

#### Download Repository

```go
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Repository branch
branch := "master"
// Local path in the file system
localPath := "/Users/frogger/code/jfrog-cli"

repositoryBranches, err := client.DownloadRepository(owner, repository, branch, localPath)
```

#### Create Webhook

```go
// Organization or username
owner := "jfrog"
// The event to watch
webhookEvent := vcsutils.Push
// VCS repository
repository := "jfrog-cli"
// Webhook on branch does not supported in GitHub and Bitbucket
branch := ""
// The URL to send the payload upon a webhook event
payloadUrl := "https://acme.jfrog.io/integration/api/v1/webhook/event"

// token - A token used to validate identity of the incoming webhook
id, token, err := client.CreateWebhook(owner, repository, branch, "https://jfrog.com", webhookEvent)
```

#### Update Webhook

```go
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Webhook on branch does not supported in GitHub and Bitbucket
branch := ""
// The URL to send the payload upon a webhook event
payloadUrl := "https://acme.jfrog.io/integration/api/v1/webhook/event"
// A token to validate identity of the webhook, created by CreateWebhook command
token := "abc123"
// The webhook ID returned from a previous CreateWebhook command
webhookId := "123"
// The event to watch
webhookEvent := vcsutils.PrCreated

err := client.UpdateWebhook(owner, repository, branch, "https://jfrog.com", token, webhookId, webhookEvent)
```

#### Delete Webhook

```go
// Organization or username
owner := "jfrog"
// GitHub repository
repository := "jfrog-cli"
// The webhook ID returned from a previous CreateWebhook command
webhookId := "123"

err := client.DeleteWebhook(owner, repository, webhookId)
```

#### Set Commit Status

```go
// One of Pass, Fail, Error, or InProgress
commitStatus := vcsclient.Pass
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// Branch or commit or tag on GitHub, commit on Bitbucket
ref := "5c05522fecf8d93a11752ff255c99fcb0f0557cd"
// Title of the commit status
title := "Xray scanning"
// Description of the commit status
description := "Run JFrog Xray scan"
// URL leads to the platform to provide more information, such as Xray scanning results
detailsUrl := "https://acme.jfrog.io/ui/xray-scan-results-url"

err := client.SetCommitStatus(commitStatus, owner, repository, ref, title, description, detailsUrl)
```

##### Create Pull Request

```go
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

err := client.CreatePullRequest(owner, repository, sourceBranch, targetBranch, title, description string)
```

### Webhook Parser

```go
// Token to authenticate incoming webhooks. If empty, signature will not be verified.
token := "abc123"
// The HTTP request of the incoming webhook
request := http.Request{}
// The VCS provider
provider := vcsutils.GitHub

webhookInfo, err := webhookparser.ParseIncomingWebhook(provider, token, request)
```
