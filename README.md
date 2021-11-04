# Froggit-Go

Froggit-Go is a Go library, allowing to perform actions on VCS providers.
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
  - [Latest Commit Hash](#get-latest-commit-hash)
  - [Add Public SSH Key](#add-public-ssh-key)
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
logger := log.Default()

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
logger := log.Default()

client, err := vcsclient.NewClientBuilder(vcsProvider).ApiEndpoint(apiEndpoint).Username(username).Token(token).Build()
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
payloadUrl := "https://acme.jfrog.io/integration/api/v1/webhook/event"

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
payloadUrl := "https://acme.jfrog.io/integration/api/v1/webhook/event"
// A token to validate identity of the webhook, created by CreateWebhook command
token := "abc123"
// The webhook ID returned by the CreateWebhook API, which created this webhook
webhookId := "123"
// The event to watch
webhookEvent := vcsutils.PrCreated

err := client.UpdateWebhook(ctx, owner, repository, branch, "https://jfrog.com", token, webhookId, webhookEvent)
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
webhookId := "123"

err := client.DeleteWebhook(ctx, owner, repository, webhookId)
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
detailsUrl := "https://acme.jfrog.io/ui/xray-scan-results-url"

err := client.SetCommitStatus(ctx, commitStatus, owner, repository, ref, title, description, detailsUrl)
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

err := client.CreatePullRequest(ctx, owner, repository, sourceBranch, targetBranch, title, description string)
```

#### Get Latest Commit Hash

```go
// Go context
ctx := context.Background()
// Organization or username
owner := "jfrog"
// VCS repository
repository := "jfrog-cli"
// VCS branch
branch := "dev"

// SHA-1 hash of the latest commit
commitHash, err := client.GetLatestCommitHash(ctx, owner, repository, branch)
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

### Webhook Parser

```go
// Go context
ctx := context.Background()
// Token to authenticate incoming webhooks. If empty, signature will not be verified.
// The token is a random key generated in the CreateWebhook command.
token := "abc123"
// The HTTP request of the incoming webhook
request := http.Request{}
// The VCS provider
provider := vcsutils.GitHub

webhookInfo, err := webhookparser.ParseIncomingWebhook(provider, token, request)
```
