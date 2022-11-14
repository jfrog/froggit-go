package vcsclient

import "errors"

var errAddSshKeyToRepositoryNotSupported = errors.New("add ssh key to repository is currently not supported for Azure Repos")
var errGetRepositoryInfoNotSupported = errors.New("get repository info is currently not supported for Azure Repos")
var errGetCommitByShaNotSupported = errors.New("get commit by SHA is currently not supported for Azure Repos")
var errCreateLabelNotSupported = errors.New("create label is currently not supported for Azure Repos")
var errGetLabelNotSupported = errors.New("get label is currently not supported for Azure Repos")
var errListPullRequestLabelsNotSupported = errors.New("get list pull request labels is currently not supported for Azure Repos")
var errUnlabelPullRequestNotSupported = errors.New("unlabel pull request is currently not supported for Azure Repos")
var errUploadCodeScanningNotSupported = errors.New("upload code scanning is currently not supported for Azure Repos")
var errCreateWebhookNotSupported = errors.New("create webhook is currently not supported for Azure Repos")
var errUpdateWebhookNotSupported = errors.New("update webhook is currently not supported for Azure Repos")
var errDeleteWebhookNotSupported = errors.New("delete webhook is currently not supported for Azure Repos")
var errSetCommitStatusNotSupported = errors.New("set commit status is currently not supported for Azure Repos")
