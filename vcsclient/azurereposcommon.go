package vcsclient

import "errors"

var errAddSshKeyToRepositoryNotSupported = errors.New("add ssh key to repository is currently not supported for Azure Repos")
var getRepositoryInfoNotSupported = errors.New("get repository info is currently not supported for Azure Repos")
var getCommitByShaNotSupported = errors.New("get commit by SHA is currently not supported for Azure Repos")
var createLabelNotSupported = errors.New("create label is currently not supported for Azure Repos")
var getLabelNotSupported = errors.New("get label is currently not supported for Azure Repos")
var listPullRequestLabelsNotSupported = errors.New("get list pull request labels is currently not supported for Azure Repos")
var unlabelPullRequestNotSupported = errors.New("unlabel pull request is currently not supported for Azure Repos")
var uploadCodeScanningNotSupported = errors.New("upload code scanning is currently not supported for Azure Repos")
var createWebhookNotSupported = errors.New("create webhook is currently not supported for Azure Repos")
var updateWebhookNotSupported = errors.New("update webhook is currently not supported for Azure Repos")
var deleteWebhookNotSupported = errors.New("delete webhook is currently not supported for Azure Repos")
var setCommitStatusNotSupported = errors.New("set commit status is currently not supported for Azure Repos")
