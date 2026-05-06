package vcsclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/ktrysmt/go-bitbucket"

	"github.com/mitchellh/mapstructure"

	"github.com/jfrog/froggit-go/vcsutils"
)

// BitbucketCloudClient API version 2.0
type BitbucketCloudClient struct {
	vcsInfo VcsInfo
	url     *url.URL
	logger  vcsutils.Log
}

// NewBitbucketCloudClient create a new BitbucketCloudClient
func NewBitbucketCloudClient(vcsInfo VcsInfo, logger vcsutils.Log) (*BitbucketCloudClient, error) {
	bitbucketClient := &BitbucketCloudClient{
		vcsInfo: vcsInfo,
		logger:  logger,
	}
	if vcsInfo.APIEndpoint != "" {
		url, err := url.Parse(vcsInfo.APIEndpoint)
		if err != nil {
			return nil, err
		}
		bitbucketClient.url = url
	}
	return bitbucketClient, nil
}

func (client *BitbucketCloudClient) buildBitbucketCloudClient(_ context.Context) (*bitbucket.Client, error) {
	var bitbucketClient *bitbucket.Client
	var err error

	if client.vcsInfo.Username == "" {
		bitbucketClient, err = bitbucket.NewOAuthbearerToken(client.vcsInfo.Token)
	} else {
		bitbucketClient, err = bitbucket.NewBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)
	}
	if err != nil {
		return nil, err
	}
	if client.url != nil {
		bitbucketClient.SetApiBaseURL(*client.url)
	}
	return bitbucketClient, nil
}

// setAuthenticationHeader sets either Basic Auth or Bearer token on an outgoing HTTP request,
// depending on whether a username was provided in the VCS info.
func (client *BitbucketCloudClient) setAuthenticationHeader(req *http.Request) {
	if client.vcsInfo.Username != "" {
		req.SetBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)
	} else {
		req.Header.Set("Authorization", "Bearer "+client.vcsInfo.Token)
	}
}

// TestConnection on Bitbucket cloud
func (client *BitbucketCloudClient) TestConnection(ctx context.Context) error {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	_, err = bitbucketClient.User.Profile()
	return err
}

// ListRepositories on Bitbucket cloud
func (client *BitbucketCloudClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	results := make(map[string][]string)
	workspaces, err := bitbucketClient.Workspaces.List()
	if err != nil {
		return nil, err
	}
	for _, workspace := range workspaces.Workspaces {
		repositoriesRes, err := bitbucketClient.Repositories.ListForAccount(&bitbucket.RepositoriesOptions{Owner: workspace.Slug})
		if err != nil {
			return nil, err
		}
		for _, repo := range repositoriesRes.Items {
			results[workspace.Slug] = append(results[workspace.Slug], repo.Slug)
		}
	}
	return results, nil
}

// ListRepositoriesByOwner on Bitbucket Cloud returns the list of repositories for the given workspace slug.
func (client *BitbucketCloudClient) ListRepositoriesByOwner(ctx context.Context, owner string) ([]string, error) {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	repositoriesRes, err := bitbucketClient.Repositories.ListForAccount(&bitbucket.RepositoriesOptions{Owner: owner})
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, repo := range repositoriesRes.Items {
		repos = append(repos, repo.Slug)
	}
	return repos, nil
}

// ListBranches on Bitbucket cloud
func (client *BitbucketCloudClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	branches, err := bitbucketClient.Repositories.Repository.ListBranches(&bitbucket.RepositoryBranchOptions{Owner: owner, RepoSlug: repository})
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(branches.Branches))
	for _, branch := range branches.Branches {
		results = append(results, branch.Name)
	}
	return results, nil
}

// AddSshKeyToRepository on Bitbucket cloud, the deploy-key is always read-only.
func (client *BitbucketCloudClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, _ Permission) (err error) {
	err = validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"key name":   keyName,
		"public key": publicKey,
	})
	if err != nil {
		return
	}
	endpoint := client.vcsInfo.APIEndpoint
	if endpoint == "" {
		endpoint = bitbucket.DEFAULT_BITBUCKET_API_BASE_URL
	}
	u := fmt.Sprintf("%s/repositories/%s/%s/deploy-keys", endpoint, owner, repository)
	addKeyRequest := bitbucketCloudAddSSHKeyRequest{
		Label: keyName,
		Key:   publicKey,
	}

	body := new(bytes.Buffer)
	err = json.NewEncoder(body).Encode(addKeyRequest)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client.setAuthenticationHeader(req)

	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	response, err := bitbucketClient.HttpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, vcsutils.DiscardResponseBody(response), response.Body.Close())
	}()

	if response.StatusCode >= 300 {
		err = fmt.Errorf("failed to add SSH key to repository: %s", response.Status)
	}
	return
}

type bitbucketCloudAddSSHKeyRequest struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type bitbucketCloudInlineCommentRequest struct {
	Content commentContent `json:"content"`
	Inline  inlineDetails  `json:"inline"`
}

// CreateWebhook on Bitbucket cloud
func (client *BitbucketCloudClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadURL string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return "", "", err
	}
	token := vcsutils.CreateToken()
	options := &bitbucket.WebhooksOptions{
		Active:   true,
		Owner:    owner,
		RepoSlug: repository,
		Url:      payloadURL + "?token=" + url.QueryEscape(token),
		Events:   getBitbucketCloudWebhookEvents(webhookEvents...),
	}
	response, err := bitbucketClient.Repositories.Webhooks.Create(options)
	if err != nil {
		return "", "", err
	}
	id, err := getBitbucketCloudWebhookID(response)
	if err != nil {
		return "", "", err
	}
	return id, token, err
}

// UpdateWebhook on Bitbucket cloud
func (client *BitbucketCloudClient) UpdateWebhook(ctx context.Context, owner, repository, _, payloadURL, token,
	webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	options := &bitbucket.WebhooksOptions{
		Active:   true,
		Uuid:     webhookID,
		Owner:    owner,
		RepoSlug: repository,
		Url:      payloadURL + "?token=" + url.QueryEscape(token),
		Events:   getBitbucketCloudWebhookEvents(webhookEvents...),
	}
	_, err = bitbucketClient.Repositories.Webhooks.Update(options)
	return err
}

// DeleteWebhook on Bitbucket cloud
func (client *BitbucketCloudClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	options := &bitbucket.WebhooksOptions{
		Uuid:     webhookID,
		Owner:    owner,
		RepoSlug: repository,
	}
	_, err = bitbucketClient.Repositories.Webhooks.Delete(options)
	return err
}

// SetCommitStatus on Bitbucket cloud
func (client *BitbucketCloudClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository,
	ref, title, description, detailsURL string) error {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	commitOptions := &bitbucket.CommitsOptions{
		Owner:    owner,
		RepoSlug: repository,
		Revision: ref,
	}
	commitStatusOptions := &bitbucket.CommitStatusOptions{
		State:       getBitbucketCommitState(commitStatus),
		Key:         title,
		Description: description,
		Url:         detailsURL,
	}
	_, err = bitbucketClient.Repositories.Commits.CreateCommitStatus(commitOptions, commitStatusOptions)
	return err
}

// GetCommitStatuses on Bitbucket cloud
func (client *BitbucketCloudClient) GetCommitStatuses(ctx context.Context, owner, repository, ref string) (status []CommitStatusInfo, err error) {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	commitOptions := &bitbucket.CommitsOptions{
		Owner:    owner,
		RepoSlug: repository,
		Revision: ref,
	}
	rawStatuses, err := bitbucketClient.Repositories.Commits.GetCommitStatuses(commitOptions)
	if err != nil {
		return nil, err
	}
	results, err := bitbucketParseCommitStatuses(rawStatuses, vcsutils.BitbucketCloud)
	if err != nil {
		return nil, err
	}
	return results, err
}

func (client *BitbucketCloudClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error {
	// TODO: Once Atlassian fixes BCLOUD-23783, Bearer tokens will work for archive downloads, and we can remove this workaround.
	// Until then, fall back to git clone when no username is provided (Bearer token auth).
	if client.vcsInfo.Username == "" {
		return client.downloadRepositoryViaGitClone(ctx, owner, repository, branch, localPath)
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	client.logger.Debug("getting Bitbucket Cloud archive link to download")
	repo, err := bitbucketClient.Repositories.Repository.Get(&bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repository,
	})
	if err != nil {
		return err
	}

	downloadLink, err := getDownloadLink(repo, branch)
	if err != nil {
		return err
	}
	client.logger.Debug("received archive url:", downloadLink)
	getRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadLink, nil)
	if err != nil {
		return err
	}
	if len(client.vcsInfo.Token) > 0 {
		client.setAuthenticationHeader(getRequest)
	}

	response, err := bitbucketClient.HttpClient.Do(getRequest)
	if err != nil {
		return err
	}
	if err = vcsutils.CheckResponseStatusWithBody(response, http.StatusOK); err != nil {
		return err
	}
	client.logger.Info(repository, vcsutils.SuccessfulRepoDownload)
	err = vcsutils.Untar(localPath, response.Body, true)
	if err != nil {
		return err
	}
	client.logger.Info(vcsutils.SuccessfulRepoExtraction)
	repositoryInfo, err := client.GetRepositoryInfo(ctx, owner, repository)
	if err != nil {
		return err
	}
	// Generate .git folder with remote details
	return vcsutils.CreateDotGitFolderWithRemote(localPath, "origin", repositoryInfo.CloneInfo.HTTP)
}

func (client *BitbucketCloudClient) GetPullRequestCommentSizeLimit() int {
	return bitbucketPrContentSizeLimit
}

func (client *BitbucketCloudClient) GetPullRequestDetailsSizeLimit() int {
	return bitbucketPrContentSizeLimit
}

// CreatePullRequest on Bitbucket cloud
func (client *BitbucketCloudClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch,
	targetBranch, title, description string) error {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	client.logger.Debug(vcsutils.CreatingPullRequest, title)
	options := &bitbucket.PullRequestsOptions{
		Owner:             owner,
		SourceRepository:  owner + "/" + repository,
		RepoSlug:          repository,
		SourceBranch:      sourceBranch,
		DestinationBranch: targetBranch,
		Title:             title,
		Description:       description,
	}
	_, err = bitbucketClient.Repositories.PullRequests.Create(options)
	return err
}

// UpdatePullRequest on Bitbucket cloud
func (client *BitbucketCloudClient) UpdatePullRequest(ctx context.Context, owner, repository, title, body, targetBranchName string, prId int, state vcsutils.PullRequestState) error {
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	client.logger.Debug(vcsutils.CreatingPullRequest, title)
	options := &bitbucket.PullRequestsOptions{
		Owner:             owner,
		SourceRepository:  owner + "/" + repository,
		RepoSlug:          repository,
		Title:             title,
		Description:       body,
		DestinationBranch: targetBranchName,
		ID:                strconv.Itoa(prId),
		States:            []string{*vcsutils.MapPullRequestState(&state)},
	}
	_, err = bitbucketClient.Repositories.PullRequests.Update(options)
	return err
}

// ListOpenPullRequestsWithBody on Bitbucket cloud
func (client *BitbucketCloudClient) ListOpenPullRequestsWithBody(ctx context.Context, owner, repository string) (res []PullRequestInfo, err error) {
	return client.getOpenPullRequests(ctx, owner, repository, true)
}

// ListOpenPullRequests on Bitbucket cloud
func (client *BitbucketCloudClient) ListOpenPullRequests(ctx context.Context, owner, repository string) (res []PullRequestInfo, err error) {
	return client.getOpenPullRequests(ctx, owner, repository, false)
}

func (client *BitbucketCloudClient) getOpenPullRequests(ctx context.Context, owner, repository string, withBody bool) (res []PullRequestInfo, err error) {
	err = validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	client.logger.Debug(vcsutils.FetchingOpenPullRequests, repository)
	options := &bitbucket.PullRequestsOptions{
		Owner:    owner,
		RepoSlug: repository,
		States:   []string{"OPEN"},
	}
	pullRequests, err := bitbucketClient.Repositories.PullRequests.Gets(options)
	if err != nil {
		return
	}
	parsedPullRequests, err := vcsutils.RemapFields[pullRequestsResponse](pullRequests, "json")
	if err != nil {
		return
	}
	return mapBitbucketCloudPullRequestToPullRequestInfo(&parsedPullRequests, withBody), nil
}

func (client *BitbucketCloudClient) GetPullRequestByID(ctx context.Context, owner, repository string, pullRequestId int) (pullRequestInfo PullRequestInfo, err error) {
	err = validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return PullRequestInfo{}, err
	}
	client.logger.Debug(vcsutils.FetchingPullRequestById, repository)
	prIdStr := strconv.Itoa(pullRequestId)
	options := &bitbucket.PullRequestsOptions{
		Owner:    owner,
		RepoSlug: repository,
		ID:       prIdStr,
	}
	pullRequestRaw, err := bitbucketClient.Repositories.PullRequests.Get(options)
	if err != nil {
		return
	}
	pullRequestDetails, err := vcsutils.RemapFields[pullRequestsDetails](pullRequestRaw, "json")
	if err != nil {
		return
	}

	sourceOwner, sourceRepository := splitBitbucketCloudRepoName(pullRequestDetails.Source.Repository.Name)
	targetOwner, targetRepository := splitBitbucketCloudRepoName(pullRequestDetails.Target.Repository.Name)

	pullRequestInfo = PullRequestInfo{
		ID:     pullRequestDetails.ID,
		Title:  pullRequestDetails.Title,
		Author: pullRequestDetails.Author.DisplayName,
		Source: BranchInfo{
			Name:       pullRequestDetails.Source.Name.Str,
			Repository: sourceRepository,
			Owner:      sourceOwner,
		},
		Target: BranchInfo{
			Name:       pullRequestDetails.Target.Name.Str,
			Repository: targetRepository,
			Owner:      targetOwner,
		},
	}
	return
}

// AddPullRequestComment on Bitbucket cloud
func (client *BitbucketCloudClient) AddPullRequestComment(ctx context.Context, owner, repository, content string, pullRequestID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "content": content})
	if err != nil {
		return err
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	options := &bitbucket.PullRequestCommentOptions{
		Owner:         owner,
		RepoSlug:      repository,
		PullRequestID: fmt.Sprint(pullRequestID),
		Content:       content,
	}
	_, err = bitbucketClient.Repositories.PullRequests.AddComment(options)
	return err
}

// AddPullRequestReviewComments on Bitbucket cloud
func (client *BitbucketCloudClient) AddPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...PullRequestComment) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return err
	}
	endpoint := client.vcsInfo.APIEndpoint
	if endpoint == "" {
		endpoint = bitbucket.DEFAULT_BITBUCKET_API_BASE_URL
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	for _, comment := range comments {
		requestBody := bitbucketCloudInlineCommentRequest{
			Content: commentContent{Raw: comment.Content},
			Inline: inlineDetails{
				To:   comment.NewEndLine,
				Path: comment.NewFilePath,
			},
		}
		body := new(bytes.Buffer)
		if err = json.NewEncoder(body).Encode(requestBody); err != nil {
			return err
		}
		u := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/comments", endpoint, owner, repository, pullRequestID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		client.setAuthenticationHeader(req)
		response, err := bitbucketClient.HttpClient.Do(req)
		if err != nil {
			return err
		}
		if closeErr := errors.Join(vcsutils.DiscardResponseBody(response), response.Body.Close()); closeErr != nil {
			return closeErr
		}
		if response.StatusCode >= 300 {
			return fmt.Errorf("%s", response.Status)
		}
	}
	return nil
}

// ListPullRequestReviewComments on Bitbucket cloud
func (client *BitbucketCloudClient) ListPullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	options := &bitbucket.PullRequestsOptions{
		Owner:    owner,
		RepoSlug: repository,
		ID:       fmt.Sprint(pullRequestID),
	}
	comments, err := bitbucketClient.Repositories.PullRequests.GetComments(options)
	if err != nil {
		return nil, err
	}
	parsedComments, err := vcsutils.RemapFields[commentsResponse](comments, "json")
	if err != nil {
		return nil, err
	}
	var result []CommentInfo
	for _, comment := range parsedComments.Values {
		if comment.Inline != nil {
			result = append(result, CommentInfo{
				ID:      comment.ID,
				Content: comment.Content.Raw,
				Created: comment.Created,
			})
		}
	}
	return result, nil
}

func (client *BitbucketCloudClient) ListPullRequestReviews(ctx context.Context, owner, repository string, pullRequestID int) ([]PullRequestReviewDetails, error) {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}

	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	options := &bitbucket.PullRequestsOptions{
		Owner:    owner,
		RepoSlug: repository,
		ID:       fmt.Sprint(pullRequestID),
	}

	comments, err := bitbucketClient.Repositories.PullRequests.GetComments(options)
	if err != nil {
		return nil, err
	}

	parsedComments, err := vcsutils.RemapFields[commentsResponse](comments, "json")
	if err != nil {
		return nil, err
	}

	var reviewInfos []PullRequestReviewDetails
	for _, comment := range parsedComments.Values {
		reviewInfos = append(reviewInfos, PullRequestReviewDetails{
			ID:          comment.ID,
			Reviewer:    comment.User.DisplayName,
			Body:        comment.Content.Raw,
			SubmittedAt: comment.Created.Format(time.RFC3339),
			CommitID:    "", // Bitbucket Cloud comments do not have a commit ID
		})
	}

	return reviewInfos, nil
}

func (client *BitbucketCloudClient) ListPullRequestsAssociatedWithCommit(ctx context.Context, owner, repository, commitSHA string) ([]PullRequestInfo, error) {
	return nil, errBitbucketListPullRequestAssociatedCommitsNotSupported
}

// ListPullRequestComments on Bitbucket cloud
func (client *BitbucketCloudClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) (res []CommentInfo, err error) {
	err = validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	options := &bitbucket.PullRequestsOptions{
		Owner:    owner,
		RepoSlug: repository,
		ID:       fmt.Sprint(pullRequestID),
	}
	comments, err := bitbucketClient.Repositories.PullRequests.GetComments(options)
	if err != nil {
		return
	}
	parsedComments, err := vcsutils.RemapFields[commentsResponse](comments, "json")
	if err != nil {
		return
	}
	return mapBitbucketCloudCommentToCommentInfo(&parsedComments), nil
}

// DeletePullRequestReviewComments on Bitbucket cloud
func (client *BitbucketCloudClient) DeletePullRequestReviewComments(ctx context.Context, owner, repository string, pullRequestID int, comments ...CommentInfo) error {
	for _, comment := range comments {
		if err := client.DeletePullRequestComment(ctx, owner, repository, pullRequestID, int(comment.ID)); err != nil {
			return err
		}
	}
	return nil
}

// DeletePullRequestComment on Bitbucket cloud
func (client *BitbucketCloudClient) DeletePullRequestComment(ctx context.Context, owner, repository string, pullRequestID, commentID int) (err error) {
	err = validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return
	}
	endpoint := client.vcsInfo.APIEndpoint
	if endpoint == "" {
		endpoint = bitbucket.DEFAULT_BITBUCKET_API_BASE_URL
	}
	u := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/comments/%d", endpoint, owner, repository, pullRequestID, commentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return
	}
	client.setAuthenticationHeader(req)
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return err
	}
	response, err := bitbucketClient.HttpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, vcsutils.DiscardResponseBody(response), response.Body.Close())
	}()
	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		err = fmt.Errorf("failed to delete comment (HTTP %d): %s", response.StatusCode, string(body))
	}
	return
}

// GetLatestCommit on Bitbucket cloud
func (client *BitbucketCloudClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"branch":     branch,
	})
	if err != nil {
		return CommitInfo{}, err
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}
	bitbucketClient.Pagelen = 1
	options := &bitbucket.CommitsOptions{
		Owner:       owner,
		RepoSlug:    repository,
		Branchortag: branch,
	}
	commits, err := bitbucketClient.Repositories.Commits.GetCommits(options)
	if err != nil {
		return CommitInfo{}, err
	}
	parsedCommits, err := vcsutils.RemapFields[commitResponse](commits, "json")
	if err != nil {
		return CommitInfo{}, err
	}
	if len(parsedCommits.Values) > 0 {
		latestCommit := parsedCommits.Values[0]
		return mapBitbucketCloudCommitToCommitInfo(latestCommit), nil
	}
	return CommitInfo{}, nil
}

// GetCommits on Bitbucket Cloud
func (client *BitbucketCloudClient) GetCommits(_ context.Context, _, _, _ string) ([]CommitInfo, error) {
	return nil, errBitbucketGetCommitsNotSupported
}

func (client *BitbucketCloudClient) GetCommitsWithQueryOptions(ctx context.Context, owner, repository string, listOptions GitCommitsQueryOptions) ([]CommitInfo, error) {
	return nil, errBitbucketGetCommitsWithOptionsNotSupported
}

// GetRepositoryInfo on Bitbucket cloud
func (client *BitbucketCloudClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository}); err != nil {
		return RepositoryInfo{}, err
	}
	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return RepositoryInfo{}, err
	}
	repo, err := bitbucketClient.Repositories.Repository.Get(&bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repository,
	})
	if err != nil {
		return RepositoryInfo{}, err
	}

	holder := struct {
		Clone []struct {
			Name string `mapstructure:"name"`
			HRef string `mapstructure:"href"`
		} `mapstructure:"clone"`
	}{}

	if err := mapstructure.Decode(repo.Links, &holder); err != nil {
		return RepositoryInfo{}, err
	}

	var info CloneInfo
	for _, link := range holder.Clone {
		switch strings.ToLower(link.Name) {
		case "https":
			info.HTTP = link.HRef
		case "ssh":
			info.SSH = link.HRef
		}
	}
	return RepositoryInfo{RepositoryVisibility: getBitbucketCloudRepositoryVisibility(repo), CloneInfo: info}, nil
}

// GetCommitBySha on Bitbucket cloud
func (client *BitbucketCloudClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"sha":        sha,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}
	options := &bitbucket.CommitsOptions{
		Owner:    owner,
		RepoSlug: repository,
		Revision: sha,
	}
	commit, err := bitbucketClient.Repositories.Commits.GetCommit(options)
	if err != nil {
		return CommitInfo{}, err
	}
	parsedCommit, err := vcsutils.RemapFields[commitDetails](commit, "json")
	if err != nil {
		return CommitInfo{}, err
	}
	return mapBitbucketCloudCommitToCommitInfo(parsedCommit), nil
}

// CreateLabel on Bitbucket cloud
func (client *BitbucketCloudClient) CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error {
	return errLabelsNotSupported
}

// GetLabel on Bitbucket cloud
func (client *BitbucketCloudClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	return nil, errLabelsNotSupported
}

// ListPullRequestLabels on Bitbucket cloud
func (client *BitbucketCloudClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	return nil, errLabelsNotSupported
}

// UnlabelPullRequest on Bitbucket cloud
func (client *BitbucketCloudClient) UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error {
	return errLabelsNotSupported
}

// UploadCodeScanning on Bitbucket cloud
func (client *BitbucketCloudClient) UploadCodeScanning(ctx context.Context, owner string, repository string, branch string, scanResults string) (string, error) {
	return "", errBitbucketCodeScanningNotSupported
}

// UploadCodeScanningWithRef on Bitbucket Cloud
func (client *BitbucketCloudClient) UploadCodeScanningWithRef(_ context.Context, _ string, _ string, _ string, _ string, _ string) (string, error) {
	return "", errBitbucketCodeScanningNotSupported
}

func (client *BitbucketCloudClient) ListAppRepositories(ctx context.Context) ([]AppRepositoryInfo, error) {
	return nil, errBitbucketListAppReposNotSupported
}

// DownloadFileFromRepo on Bitbucket cloud
func (client *BitbucketCloudClient) DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) ([]byte, int, error) {
	return nil, 0, errBitbucketDownloadFileFromRepoNotSupported
}

// GetRepositoryEnvironmentInfo on Bitbucket cloud
func (client *BitbucketCloudClient) GetRepositoryEnvironmentInfo(ctx context.Context, owner, repository, name string) (RepositoryEnvironmentInfo, error) {
	return RepositoryEnvironmentInfo{}, errBitbucketGetRepoEnvironmentInfoNotSupported
}

func (client *BitbucketCloudClient) GetModifiedFiles(ctx context.Context, owner, repository, refBefore, refAfter string) ([]string, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"refBefore":  refBefore,
		"refAfter":   refAfter,
	})
	if err != nil {
		return nil, err
	}

	bitbucketClient, err := client.buildBitbucketCloudClient(ctx)
	if err != nil {
		return nil, err
	}
	options := &bitbucket.DiffStatOptions{
		Owner:    owner,
		RepoSlug: repository,
		// We use 2 dots for spec because of the case described at the page:
		// https://developer.atlassian.com/cloud/bitbucket/rest/api-group-commits/#two-commit-spec
		// As there is no `topic` set it will be treated as `refAfter...refBefore` actually.
		Spec:    refAfter + ".." + refBefore,
		Renames: true,
		Merge:   true,
	}

	fileNamesSet := datastructures.MakeSet[string]()
	nextPage := 1

	for nextPage > 0 {
		options.PageNum = nextPage
		diffStatRes, err := bitbucketClient.Repositories.Diff.GetDiffStat(options)
		if err != nil {
			return nil, err
		}

		if diffStatRes.Next == "" {
			nextPage = -1
		} else {
			nextPage++
		}

		for _, diffStat := range diffStatRes.DiffStats {
			if path, ok := diffStat.New["path"].(string); ok {
				fileNamesSet.Add(path)
			}
			if path, ok := diffStat.Old["path"].(string); ok {
				fileNamesSet.Add(path)
			}
		}
	}
	_ = fileNamesSet.Remove("") // Make sure there are no blank filepath.
	fileNamesList := fileNamesSet.ToSlice()
	sort.Strings(fileNamesList)
	return fileNamesList, nil
}

func (client *BitbucketCloudClient) CreateBranch(ctx context.Context, owner, repository, sourceBranch, newBranch string) error {
	return errBitbucketCreateBranchNotSupported
}

func (client *BitbucketCloudClient) AllowWorkflows(ctx context.Context, owner string) error {
	return errBitbucketAllowWorkflowsNotSupported
}

func (client *BitbucketCloudClient) AddOrganizationSecret(ctx context.Context, owner, secretName, secretValue string) error {
	return errBitbucketAddOrganizationSecretNotSupported
}

func (client *BitbucketCloudClient) CreateOrgVariable(ctx context.Context, owner, variableName, variableValue string) error {
	return errBitbucketCreateOrgVariableNotSupported
}

func (client *BitbucketCloudClient) CommitAndPushFiles(ctx context.Context, owner, repo, sourceBranch, commitMessage, authorName, authorEmail string, files []FileToCommit) error {
	return errBitbucketCommitAndPushFilesNotSupported
}

func (client *BitbucketCloudClient) GetRepoCollaborators(ctx context.Context, owner, repo, affiliation, permission string) ([]string, error) {
	return nil, errBitbucketGetRepoCollaboratorsNotSupported
}

func (client *BitbucketCloudClient) GetRepoTeamsByPermissions(ctx context.Context, owner, repo string, permissions []string) ([]int64, error) {
	return nil, errBitbucketGetRepoTeamsByPermissionsNotSupported
}

func (client *BitbucketCloudClient) CreateOrUpdateEnvironment(ctx context.Context, owner, repo, envName string, teams []int64, users []string) error {
	return errBitbucketCreateOrUpdateEnvironmentNotSupported
}

func (client *BitbucketCloudClient) MergePullRequest(ctx context.Context, owner, repo string, prNumber int, commitMessage string) error {
	return errBitbucketMergePullRequestNotSupported
}

func (client *BitbucketCloudClient) CreatePullRequestDetailed(ctx context.Context, owner, repository, sourceBranch, targetBranch, title, description string) (CreatedPullRequestInfo, error) {
	return CreatedPullRequestInfo{}, errBitbucketCreatePullRequestDetailedNotSupported
}

func (client *BitbucketCloudClient) UploadSnapshotToDependencyGraph(ctx context.Context, owner, repo string, snapshot *SbomSnapshot) error {
	return errBitbucketUploadSnapshotToDependencyGraphNotSupported
}

type pullRequestsResponse struct {
	Values []pullRequestsDetails `json:"values"`
}

type pullRequestsDetails struct {
	ID     int64             `json:"id"`
	Title  string            `json:"title"`
	Body   string            `json:"description"`
	Author Author            `json:"author"`
	Source pullRequestBranch `json:"source"`
	Target pullRequestBranch `json:"destination"`
}

type Author struct {
	DisplayName string `json:"display_name"`
}

type pullRequestBranch struct {
	Name struct {
		Str string `json:"name"`
	} `json:"branch"`
	Repository pullRequestRepository `json:"repository"`
}

type pullRequestRepository struct {
	Name string `json:"full_name"`
}

type commentsResponse struct {
	Values []commentDetails `json:"values"`
}

type commentDetails struct {
	ID        int64          `json:"id"`
	User      user           `json:"user"`
	IsDeleted bool           `json:"deleted"`
	Content   commentContent `json:"content"`
	Created   time.Time      `json:"created_on"`
	Inline    *inlineDetails `json:"inline,omitempty"`
}

type commentContent struct {
	Raw string `json:"raw"`
}

type inlineDetails struct {
	To   int    `json:"to"`
	Path string `json:"path"`
}

type commitResponse struct {
	Values []commitDetails `json:"values"`
}

type commitDetails struct {
	Hash    string    `json:"hash"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
	Author  struct {
		User user `json:"user"`
	} `json:"author"`
	Links struct {
		Self link `json:"self"`
	} `json:"links"`
	Parents []struct {
		Hash string `json:"hash"`
	} `json:"parents"`
}

type user struct {
	DisplayName string `json:"display_name"`
}
type link struct {
	Href string `json:"href"`
}

// Extract the webhook ID from the webhook create response
func getBitbucketCloudWebhookID(r interface{}) (string, error) {
	webhook := &bitbucket.WebhooksOptions{}
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(b, &webhook)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(strings.TrimLeft(webhook.Uuid, "{"), "}"), nil
}

// Get varargs of webhook events and return a slice of Bitbucket cloud webhook events
func getBitbucketCloudWebhookEvents(webhookEvents ...vcsutils.WebhookEvent) []string {
	events := datastructures.MakeSet[string]()
	for _, event := range webhookEvents {
		switch event {
		case vcsutils.PrOpened:
			events.Add("pullrequest:created")
		case vcsutils.PrEdited:
			events.Add("pullrequest:updated")
		case vcsutils.PrRejected:
			events.Add("pullrequest:rejected")
		case vcsutils.PrMerged:
			events.Add("pullrequest:fulfilled")
		case vcsutils.Push, vcsutils.TagPushed, vcsutils.TagRemoved:
			events.Add("repo:push")
		}
	}
	return events.ToSlice()
}

// The get repository request returns HTTP link to the repository - extract the link from the response.
func getDownloadLink(repo *bitbucket.Repository, branch string) (string, error) {
	repositoryHTMLLinks := &link{}
	b, err := json.Marshal(repo.Links["html"])
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(b, repositoryHTMLLinks)
	if err != nil {
		return "", err
	}
	htmlLink := repositoryHTMLLinks.Href
	if htmlLink == "" {
		return "", fmt.Errorf("couldn't find repository HTML link: %s", repo.Links["html"])
	}
	return htmlLink + "/get/" + branch + ".tar.gz", err
}

func mapBitbucketCloudCommitToCommitInfo(parsedCommit commitDetails) CommitInfo {
	parents := make([]string, len(parsedCommit.Parents))
	for i, p := range parsedCommit.Parents {
		parents[i] = p.Hash
	}
	return CommitInfo{
		Hash:          parsedCommit.Hash,
		AuthorName:    parsedCommit.Author.User.DisplayName,
		CommitterName: "", // not provided
		Url:           parsedCommit.Links.Self.Href,
		Timestamp:     parsedCommit.Date.UTC().Unix(),
		Message:       parsedCommit.Message,
		ParentHashes:  parents,
	}
}

func mapBitbucketCloudCommentToCommentInfo(parsedComments *commentsResponse) []CommentInfo {
	comments := make([]CommentInfo, len(parsedComments.Values))
	for i, comment := range parsedComments.Values {
		comments[i] = CommentInfo{
			ID:      comment.ID,
			Content: comment.Content.Raw,
			Created: comment.Created,
		}
	}
	return comments
}

func mapBitbucketCloudPullRequestToPullRequestInfo(parsedPullRequests *pullRequestsResponse, withBody bool) []PullRequestInfo {
	pullRequests := make([]PullRequestInfo, len(parsedPullRequests.Values))
	for i, pullRequest := range parsedPullRequests.Values {
		var body string
		if withBody {
			body = pullRequest.Body
		}
		pullRequests[i] = PullRequestInfo{
			ID:     pullRequest.ID,
			Title:  pullRequest.Title,
			Body:   body,
			Author: pullRequest.Author.DisplayName,
			Source: BranchInfo{
				Name:       pullRequest.Source.Name.Str,
				Repository: pullRequest.Source.Repository.Name,
			},
			Target: BranchInfo{
				Name:       pullRequest.Target.Name.Str,
				Repository: pullRequest.Target.Repository.Name,
			},
		}
	}
	return pullRequests
}

func getBitbucketCloudRepositoryVisibility(repo *bitbucket.Repository) RepositoryVisibility {
	if repo.Is_private {
		return Private
	}
	return Public
}

// Bitbucket cloud repository name is a combination of workspace/repository
// Return the two separate elements
func splitBitbucketCloudRepoName(name string) (string, string) {
	split := strings.Split(name, "/")
	if len(split) < 2 {
		return "", ""
	}
	return split[0], split[1]
}

// Clones the repository using git with x-token-auth authentication.
// This is a workaround for BCLOUD-23783: Bitbucket Cloud archive downloads do not support Bearer
// token auth. Remove this fallback once Atlassian resolves BCLOUD-23783.
func (client *BitbucketCloudClient) downloadRepositoryViaGitClone(ctx context.Context, owner, repository, branch, localPath string) (err error) {
	client.logger.Debug("Using git clone fallback (BCLOUD-23783 workaround: Bearer tokens not supported for archive downloads)")
	cloneURL := fmt.Sprintf("https://bitbucket.org/%s/%s.git", owner, repository)

	tempDir, err := os.MkdirTemp("", "bitbucket-clone-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			err = errors.Join(err, removeErr)
		}
	}()

	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, cloneURL, tempDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	if client.vcsInfo.Token != "" {
		creds := base64.StdEncoding.EncodeToString([]byte("x-token-auth:" + client.vcsInfo.Token))
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.extraHeader",
			fmt.Sprintf("GIT_CONFIG_VALUE_0=Authorization: Basic %s", creds),
		)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if client.vcsInfo.Token != "" {
			stderrStr = strings.ReplaceAll(stderrStr, client.vcsInfo.Token, "***")
		}
		return fmt.Errorf("git clone failed: %w, stderr: %s", err, stderrStr)
	}
	client.logger.Info(repository, vcsutils.SuccessfulRepoDownload)

	if err = os.RemoveAll(filepath.Join(tempDir, ".git")); err != nil {
		client.logger.Debug("Failed to remove .git directory:", err)
	}
	if err = biutils.CopyDir(tempDir, localPath, true, nil); err != nil {
		return fmt.Errorf("failed to copy repository contents: %w", err)
	}
	client.logger.Info(vcsutils.SuccessfulRepoExtraction)

	repositoryInfo, err := client.GetRepositoryInfo(ctx, owner, repository)
	if err != nil {
		return err
	}
	return vcsutils.CreateDotGitFolderWithRemote(localPath, "origin", repositoryInfo.CloneInfo.HTTP)
}
