package vcsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/ktrysmt/go-bitbucket"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/jfrog/froggit-go/vcsutils"
)

// BitbucketCloudClient API version 2.0
type BitbucketCloudClient struct {
	vcsInfo VcsInfo
	url     *url.URL
	logger  Log
}

// NewBitbucketCloudClient create a new BitbucketCloudClient
func NewBitbucketCloudClient(vcsInfo VcsInfo, logger Log) (*BitbucketCloudClient, error) {
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

func (client *BitbucketCloudClient) buildBitbucketCloudClient(_ context.Context) *bitbucket.Client {
	bitbucketClient := bitbucket.NewBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)
	if client.url != nil {
		bitbucketClient.SetApiBaseURL(*client.url)
	}
	return bitbucketClient
}

// TestConnection on Bitbucket cloud
func (client *BitbucketCloudClient) TestConnection(ctx context.Context) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	_, err := bitbucketClient.User.Profile()
	return err
}

// ListRepositories on Bitbucket cloud
func (client *BitbucketCloudClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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

// ListBranches on Bitbucket cloud
func (client *BitbucketCloudClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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
	req.SetBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)

	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	response, err := bitbucketClient.HttpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, vcsutils.DiscardResponseBody(response), response.Body.Close())
	}()

	if response.StatusCode >= 300 {
		err = fmt.Errorf(response.Status)
	}
	return
}

type bitbucketCloudAddSSHKeyRequest struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// CreateWebhook on Bitbucket cloud
func (client *BitbucketCloudClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadURL string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	options := &bitbucket.WebhooksOptions{
		Active:   true,
		Uuid:     webhookID,
		Owner:    owner,
		RepoSlug: repository,
		Url:      payloadURL + "?token=" + url.QueryEscape(token),
		Events:   getBitbucketCloudWebhookEvents(webhookEvents...),
	}
	_, err := bitbucketClient.Repositories.Webhooks.Update(options)
	return err
}

// DeleteWebhook on Bitbucket cloud
func (client *BitbucketCloudClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	options := &bitbucket.WebhooksOptions{
		Uuid:     webhookID,
		Owner:    owner,
		RepoSlug: repository,
	}
	_, err := bitbucketClient.Repositories.Webhooks.Delete(options)
	return err
}

// SetCommitStatus on Bitbucket cloud
func (client *BitbucketCloudClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, owner, repository,
	ref, title, description, detailsURL string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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
	_, err := bitbucketClient.Repositories.Commits.CreateCommitStatus(commitOptions, commitStatusOptions)
	return err
}

// GetCommitStatuses on Bitbucket cloud
func (client *BitbucketCloudClient) GetCommitStatuses(ctx context.Context, owner, repository, ref string) (status []CommitStatusInfo, err error) {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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

// DownloadRepository on Bitbucket cloud
func (client *BitbucketCloudClient) DownloadRepository(ctx context.Context, owner, repository, branch,
	localPath string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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
	if len(client.vcsInfo.Username) > 0 || len(client.vcsInfo.Token) > 0 {
		getRequest.SetBasicAuth(client.vcsInfo.Username, client.vcsInfo.Token)
	}

	response, err := bitbucketClient.HttpClient.Do(getRequest)
	if err != nil {
		return err
	}
	if err = vcsutils.CheckResponseStatusWithBody(response, http.StatusOK); err != nil {
		return err
	}
	client.logger.Info(repository, successfulRepoDownload)
	err = vcsutils.Untar(localPath, response.Body, true)
	if err != nil {
		return err
	}
	client.logger.Info(successfulRepoExtraction)
	repositoryInfo, err := client.GetRepositoryInfo(ctx, owner, repository)
	if err != nil {
		return err
	}
	// Generate .git folder with remote details
	return vcsutils.CreateDotGitFolderWithRemote(localPath, "origin", repositoryInfo.CloneInfo.HTTP)
}

// CreatePullRequest on Bitbucket cloud
func (client *BitbucketCloudClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch,
	targetBranch, title, description string) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	client.logger.Debug(creatingPullRequest, title)
	options := &bitbucket.PullRequestsOptions{
		Owner:             owner,
		SourceRepository:  owner + "/" + repository,
		RepoSlug:          repository,
		SourceBranch:      sourceBranch,
		DestinationBranch: targetBranch,
		Title:             title,
		Description:       description,
	}
	_, err := bitbucketClient.Repositories.PullRequests.Create(options)
	return err
}

// UpdatePullRequest on Bitbucket cloud
func (client *BitbucketCloudClient) UpdatePullRequest(ctx context.Context, owner, repository, title, body, targetBranchName string, prId int, state vcsutils.PullRequestState) error {
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	client.logger.Debug(creatingPullRequest, title)
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
	_, err := bitbucketClient.Repositories.PullRequests.Update(options)
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
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	client.logger.Debug(fetchingOpenPullRequests, repository)
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
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	client.logger.Debug(fetchingPullRequestById, repository)
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
		ID: pullRequestDetails.ID,
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
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
	options := &bitbucket.PullRequestCommentOptions{
		Owner:         owner,
		RepoSlug:      repository,
		PullRequestID: fmt.Sprint(pullRequestID),
		Content:       content,
	}
	_, err = bitbucketClient.Repositories.PullRequests.AddComment(options)
	return err
}

// ListPullRequestComments on Bitbucket cloud
func (client *BitbucketCloudClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) (res []CommentInfo, err error) {
	err = validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository})
	if err != nil {
		return nil, err
	}
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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

// DeletePullRequestComment on Bitbucket cloud
func (client *BitbucketCloudClient) DeletePullRequestComment(_ context.Context, _, _ string, _, _ int) error {
	return nil
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
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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

// GetRepositoryInfo on Bitbucket cloud
func (client *BitbucketCloudClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository}); err != nil {
		return RepositoryInfo{}, err
	}
	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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

	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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

	bitbucketClient := client.buildBitbucketCloudClient(ctx)
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

type pullRequestsResponse struct {
	Values []pullRequestsDetails `json:"values"`
}

type pullRequestsDetails struct {
	ID     int64             `json:"id"`
	Body   string            `json:"description"`
	Source pullRequestBranch `json:"source"`
	Target pullRequestBranch `json:"destination"`
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
}

type commentContent struct {
	Raw string `json:"raw"`
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
			ID:   pullRequest.ID,
			Body: body,
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
