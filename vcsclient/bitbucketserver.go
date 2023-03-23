package vcsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/datastructures"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/oauth2"
)

// BitbucketServerClient API version 1.0
type BitbucketServerClient struct {
	vcsInfo VcsInfo
	logger  Log
}

// NewBitbucketServerClient create a new BitbucketServerClient
func NewBitbucketServerClient(vcsInfo VcsInfo, logger Log) (*BitbucketServerClient, error) {
	bitbucketServerClient := &BitbucketServerClient{
		vcsInfo: vcsInfo,
		logger:  logger,
	}
	return bitbucketServerClient, nil
}

func (client *BitbucketServerClient) buildBitbucketClient(ctx context.Context) (*bitbucketv1.DefaultApiService, error) {
	// Bitbucket API Endpoint ends with '/rest'
	if !strings.HasSuffix(client.vcsInfo.APIEndpoint, "/rest") {
		client.vcsInfo.APIEndpoint += "/rest"
	}

	bbClient := bitbucketv1.NewAPIClient(ctx, &bitbucketv1.Configuration{
		HTTPClient: client.buildHTTPClient(ctx),
		BasePath:   client.vcsInfo.APIEndpoint,
	})
	return bbClient.DefaultApi, nil
}

func (client *BitbucketServerClient) buildHTTPClient(ctx context.Context) *http.Client {
	httpClient := &http.Client{}
	if client.vcsInfo.Token != "" {
		httpClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: client.vcsInfo.Token}))
	}
	return httpClient
}

// TestConnection on Bitbucket server
func (client *BitbucketServerClient) TestConnection(ctx context.Context) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}

	options := map[string]interface{}{"limit": 1}
	_, err = bitbucketClient.GetUsers(options)
	return err
}

// ListRepositories on Bitbucket server
func (client *BitbucketServerClient) ListRepositories(ctx context.Context) (map[string][]string, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := client.listProjects(bitbucketClient)
	if err != nil {
		return nil, err
	}

	results := make(map[string][]string)
	for _, project := range projects {
		var apiResponse *bitbucketv1.APIResponse
		for isLastReposPage, nextReposPageStart := true, 0; isLastReposPage; isLastReposPage, nextReposPageStart = bitbucketv1.HasNextPage(apiResponse) {
			// Get all repositories for which the authenticated user has the REPO_READ permission
			apiResponse, err = bitbucketClient.GetRepositoriesWithOptions(project, createPaginationOptions(nextReposPageStart))
			if err != nil {
				return nil, err
			}

			repos, err := bitbucketv1.GetRepositoriesResponse(apiResponse)
			if err != nil {
				return nil, err
			}
			for _, repo := range repos {
				results[project] = append(results[project], repo.Slug)
			}
		}
	}
	return results, nil
}

// ListBranches on Bitbucket server
func (client *BitbucketServerClient) ListBranches(ctx context.Context, owner, repository string) ([]string, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}
	var results []string
	var apiResponse *bitbucketv1.APIResponse
	for isLastPage, nextPageStart := true, 0; isLastPage; isLastPage, nextPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = bitbucketClient.GetBranches(owner, repository, createPaginationOptions(nextPageStart))
		if err != nil {
			return nil, err
		}
		branches, err := bitbucketv1.GetBranchesResponse(apiResponse)
		if err != nil {
			return nil, err
		}

		for _, branch := range branches {
			results = append(results, branch.ID)
		}
	}

	return results, nil
}

// AddSshKeyToRepository on Bitbucket server
func (client *BitbucketServerClient) AddSshKeyToRepository(ctx context.Context, owner, repository, keyName, publicKey string, permission Permission) error {
	// https://docs.atlassian.com/bitbucket-server/rest/5.16.0/bitbucket-ssh-rest.html
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"key name":   keyName,
		"public key": publicKey,
	})
	if err != nil {
		return err
	}

	accessPermission := "REPO_READ"
	if permission == ReadWrite {
		accessPermission = "REPO_WRITE"
	}

	url := fmt.Sprintf("%s/keys/1.0/projects/%s/repos/%s/ssh", client.vcsInfo.APIEndpoint, owner, repository)
	addKeyRequest := bitbucketServerAddSSHKeyRequest{
		Key:        bitbucketServerSSHKey{Text: publicKey, Label: keyName},
		Permission: accessPermission,
	}

	body := new(bytes.Buffer)
	err = json.NewEncoder(body).Encode(addKeyRequest)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := client.buildHTTPClient(ctx)
	response, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode >= 300 {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %v, body: %s", response.Status, bodyBytes)
	}
	_ = vcsutils.DiscardResponseBody(response)
	return nil
}

type bitbucketServerAddSSHKeyRequest struct {
	Key        bitbucketServerSSHKey `json:"key"`
	Permission string                `json:"permission"`
}

type bitbucketServerSSHKey struct {
	Text  string `json:"text"`
	Label string `json:"label"`
}

// CreateWebhook on Bitbucket server
func (client *BitbucketServerClient) CreateWebhook(ctx context.Context, owner, repository, _, payloadURL string,
	webhookEvents ...vcsutils.WebhookEvent) (string, string, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return "", "", err
	}
	token := vcsutils.CreateToken()
	hook := createBitbucketServerHook(token, payloadURL, webhookEvents...)
	response, err := bitbucketClient.CreateWebhook(owner, repository, hook, []string{})
	if err != nil {
		return "", "", err
	}
	webhoodID, err := getBitbucketServerWebhookID(response)
	if err != nil {
		return "", "", err
	}
	return webhoodID, token, err
}

// UpdateWebhook on Bitbucket server
func (client *BitbucketServerClient) UpdateWebhook(ctx context.Context, owner, repository, _, payloadURL, token,
	webhookID string, webhookEvents ...vcsutils.WebhookEvent) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	webhookIDInt32, err := strconv.ParseInt(webhookID, 10, 32)
	if err != nil {
		return err
	}
	hook := createBitbucketServerHook(token, payloadURL, webhookEvents...)
	_, err = bitbucketClient.UpdateWebhook(owner, repository, int32(webhookIDInt32), hook, []string{})
	return err
}

// DeleteWebhook on Bitbucket server
func (client *BitbucketServerClient) DeleteWebhook(ctx context.Context, owner, repository, webhookID string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	webhookIDInt32, err := strconv.ParseInt(webhookID, 10, 32)
	if err != nil {
		return err
	}
	_, err = bitbucketClient.DeleteWebhook(owner, repository, int32(webhookIDInt32))
	return err
}

// SetCommitStatus on Bitbucket server
func (client *BitbucketServerClient) SetCommitStatus(ctx context.Context, commitStatus CommitStatus, _, _, ref, title,
	description, detailsURL string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	_, err = bitbucketClient.SetCommitStatus(ref, bitbucketv1.BuildStatus{
		State:       getBitbucketCommitState(commitStatus),
		Key:         title,
		Description: description,
		Url:         detailsURL,
	})
	return err
}

// GetCommitStatuses on Bitbucket server
func (client *BitbucketServerClient) GetCommitStatuses(ctx context.Context, owner, repository, ref string) (status []CommitStatusInfo, err error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := bitbucketClient.GetCommitStatus(ref)
	if err != nil {
		return nil, err
	}
	results, err := BitbucketParseCommitStatuses(response.Values)
	if err != nil {
		return nil, err
	}
	return results, err
}

// DownloadRepository on Bitbucket server
func (client *BitbucketServerClient) DownloadRepository(ctx context.Context, owner, repository, branch, localPath string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	params := map[string]interface{}{"format": "tgz"}
	branch = strings.TrimSpace(branch)
	if branch != "" {
		params["at"] = branch
	}
	response, err := bitbucketClient.GetArchive(owner, repository, params)
	if err != nil {
		return err
	}
	client.logger.Info(repository, successfulRepoDownload)
	err = vcsutils.Untar(localPath, bytes.NewReader(response.Payload), false)
	if err != nil {
		return err
	}
	client.logger.Info(successfulRepoExtraction)
	// Generate .git folder with remote details
	return vcsutils.CreateDotGitFolderWithRemote(
		localPath,
		vcsutils.RemoteName,
		vcsutils.GetGenericGitRemoteUrl(fmt.Sprintf("%s/scm", strings.TrimSuffix(client.vcsInfo.APIEndpoint, "/rest")), owner, repository))
}

// CreatePullRequest on Bitbucket server
func (client *BitbucketServerClient) CreatePullRequest(ctx context.Context, owner, repository, sourceBranch, targetBranch,
	title, description string) error {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	bitbucketRepo := &bitbucketv1.Repository{
		Slug: repository,
		Project: &bitbucketv1.Project{
			Key: owner,
		},
	}
	options := bitbucketv1.PullRequest{
		Title:       title,
		Description: description,
		FromRef: bitbucketv1.PullRequestRef{
			ID:         vcsutils.AddBranchPrefix(sourceBranch),
			Repository: *bitbucketRepo,
		},
		ToRef: bitbucketv1.PullRequestRef{
			ID:         vcsutils.AddBranchPrefix(targetBranch),
			Repository: *bitbucketRepo,
		},
	}
	_, err = bitbucketClient.CreatePullRequest(owner, repository, options)
	return err
}

// ListOpenPullRequests on Bitbucket server
func (client *BitbucketServerClient) ListOpenPullRequests(ctx context.Context, owner, repository string) ([]PullRequestInfo, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}
	var results []PullRequestInfo
	var apiResponse *bitbucketv1.APIResponse
	for isLastPage, nextPageStart := true, 0; isLastPage; isLastPage, nextPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = bitbucketClient.GetPullRequestsPage(owner, repository, createPaginationOptions(nextPageStart))
		if err != nil {
			return nil, err
		}
		pullRequests, err := bitbucketv1.GetPullRequestsResponse(apiResponse)
		if err != nil {
			return nil, err
		}
		for _, pullRequest := range pullRequests {
			if pullRequest.Open {
				results = append(results, PullRequestInfo{
					ID: int64(pullRequest.ID),
					Source: BranchInfo{
						Name:       pullRequest.FromRef.ID,
						Repository: pullRequest.FromRef.Repository.Slug},
					Target: BranchInfo{
						Name:       pullRequest.ToRef.ID,
						Repository: pullRequest.ToRef.Repository.Slug},
				})
			}
		}
	}
	return results, nil
}

// AddPullRequestComment on Bitbucket server
func (client *BitbucketServerClient) AddPullRequestComment(ctx context.Context, owner, repository, content string, pullRequestID int) error {
	err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository, "content": content})
	if err != nil {
		return err
	}
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return err
	}
	_, err = bitbucketClient.CreatePullRequestComment(owner, repository, pullRequestID, bitbucketv1.Comment{
		Text: content,
	}, []string{"application/json"})

	return err
}

// ListPullRequestComments on Bitbucket server
func (client *BitbucketServerClient) ListPullRequestComments(ctx context.Context, owner, repository string, pullRequestID int) ([]CommentInfo, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}
	var results []CommentInfo
	var apiResponse *bitbucketv1.APIResponse
	for isLastPage, nextPageStart := true, 0; isLastPage; isLastPage, nextPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = bitbucketClient.GetActivities(owner, repository, int64(pullRequestID), createPaginationOptions(nextPageStart))
		if err != nil {
			return nil, err
		}
		activities, err := bitbucketv1.GetActivitiesResponse(apiResponse)
		if err != nil {
			return nil, err
		}
		for _, activity := range activities.Values {
			// Add activity only if from type new comment.
			if activity.Action == "COMMENTED" && activity.CommentAction == "ADDED" {
				results = append(results, CommentInfo{
					ID:      int64(activity.Comment.ID),
					Created: time.Unix(activity.Comment.CreatedDate, 0),
					Content: activity.Comment.Text,
				})
			}
		}
	}
	return results, nil
}

type projectsResponse struct {
	Values []struct {
		Key string `json:"key,omitempty"`
	} `json:"values,omitempty"`
}

// GetLatestCommit on Bitbucket server
func (client *BitbucketServerClient) GetLatestCommit(ctx context.Context, owner, repository, branch string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"branch":     branch,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	options := map[string]interface{}{
		"limit": 1,
		"until": branch,
	}
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}

	apiResponse, err := bitbucketClient.GetCommits(owner, repository, options)
	if err != nil {
		return CommitInfo{}, err
	}
	commits, err := bitbucketv1.GetCommitsResponse(apiResponse)
	if err != nil {
		return CommitInfo{}, err
	}
	if len(commits) > 0 {
		latestCommit := commits[0]
		return client.mapBitbucketServerCommitToCommitInfo(latestCommit, owner, repository), nil
	}
	return CommitInfo{}, nil
}

// GetRepositoryInfo on Bitbucket server
func (client *BitbucketServerClient) GetRepositoryInfo(ctx context.Context, owner, repository string) (RepositoryInfo, error) {
	if err := validateParametersNotBlank(map[string]string{"owner": owner, "repository": repository}); err != nil {
		return RepositoryInfo{}, err
	}

	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return RepositoryInfo{}, err
	}

	repo, err := bitbucketClient.GetRepository(owner, repository)
	if err != nil {
		return RepositoryInfo{}, err
	}

	holder := struct {
		Links struct {
			Clone []struct {
				Name string `mapstructure:"name"`
				HRef string `mapstructure:"href"`
			} `mapstructure:"clone"`
		} `mapstructure:"links"`
		Public bool `mapstructure:"public"`
	}{}

	if err := mapstructure.Decode(repo.Values, &holder); err != nil {
		return RepositoryInfo{}, err
	}

	var info CloneInfo
	for _, cloneLink := range holder.Links.Clone {
		switch cloneLink.Name {
		case "http":
			info.HTTP = cloneLink.HRef
		case "ssh":
			info.SSH = cloneLink.HRef
		}
	}

	return RepositoryInfo{RepositoryVisibility: getBitbucketServerRepositoryVisibility(holder.Public), CloneInfo: info}, nil
}

// GetCommitBySha on Bitbucket server
func (client BitbucketServerClient) GetCommitBySha(ctx context.Context, owner, repository, sha string) (CommitInfo, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"sha":        sha,
	})
	if err != nil {
		return CommitInfo{}, err
	}

	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return CommitInfo{}, err
	}

	apiResponse, err := bitbucketClient.GetCommit(owner, repository, sha, nil)
	if err != nil {
		return CommitInfo{}, err
	}
	commit := bitbucketv1.Commit{}
	err = unmarshalAPIResponseValues(apiResponse, &commit)
	if err != nil {
		return CommitInfo{}, err
	}
	return client.mapBitbucketServerCommitToCommitInfo(commit, owner, repository), nil
}

// CreateLabel on Bitbucket server
func (client BitbucketServerClient) CreateLabel(ctx context.Context, owner, repository string, labelInfo LabelInfo) error {
	return errLabelsNotSupported
}

// GetLabel on Bitbucket server
func (client *BitbucketServerClient) GetLabel(ctx context.Context, owner, repository, name string) (*LabelInfo, error) {
	return nil, errLabelsNotSupported
}

// ListPullRequestLabels on Bitbucket server
func (client *BitbucketServerClient) ListPullRequestLabels(ctx context.Context, owner, repository string, pullRequestID int) ([]string, error) {
	return nil, errLabelsNotSupported
}

// UnlabelPullRequest on Bitbucket server
func (client *BitbucketServerClient) UnlabelPullRequest(ctx context.Context, owner, repository, name string, pullRequestID int) error {
	return errLabelsNotSupported
}

// GetRepositoryEnvironmentInfo on Bitbucket server
func (client *BitbucketServerClient) GetRepositoryEnvironmentInfo(ctx context.Context, owner, repository, name string) (RepositoryEnvironmentInfo, error) {
	return RepositoryEnvironmentInfo{}, errBitbucketGetRepoEnvironmentInfoNotSupported
}

// Get all projects for which the authenticated user has the PROJECT_VIEW permission
func (client *BitbucketServerClient) listProjects(bitbucketClient *bitbucketv1.DefaultApiService) ([]string, error) {
	var apiResponse *bitbucketv1.APIResponse
	var err error
	var projects []string
	for isLastProjectsPage, nextProjectsPageStart := true, 0; isLastProjectsPage; isLastProjectsPage, nextProjectsPageStart = bitbucketv1.HasNextPage(apiResponse) {
		apiResponse, err = bitbucketClient.GetProjects(createPaginationOptions(nextProjectsPageStart))
		if err != nil {
			return nil, err
		}
		projectsResponse := &projectsResponse{}
		err = unmarshalAPIResponseValues(apiResponse, projectsResponse)
		if err != nil {
			return nil, err
		}
		for _, project := range projectsResponse.Values {
			projects = append(projects, project.Key)
		}
	}
	// Add user's private project
	username := apiResponse.Header.Get("X-Ausername")
	if username == "" {
		return []string{}, errors.New("X-Ausername header is missing")
	}
	// project keys are upper case
	projects = append(projects, "~"+strings.ToUpper(username))
	return projects, nil
}

// DownloadFileFromRepo on Bitbucket server
func (client *BitbucketServerClient) DownloadFileFromRepo(ctx context.Context, owner, repository, branch, path string) ([]byte, int, error) {
	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, 0, err
	}
	resp, err := bitbucketClient.GetContent_11(owner, repository, path, map[string]interface{}{"at": branch})
	if err != nil {
		return nil, 0, err
	}
	return resp.Payload, resp.StatusCode, err
}

func createPaginationOptions(nextPageStart int) map[string]interface{} {
	return map[string]interface{}{"start": nextPageStart}
}

func unmarshalAPIResponseValues(response *bitbucketv1.APIResponse, target interface{}) error {
	responseBytes, err := json.Marshal(response.Values)
	if err != nil {
		return err
	}
	return json.Unmarshal(responseBytes, &target)
}

func getBitbucketServerWebhookID(r *bitbucketv1.APIResponse) (string, error) {
	webhook := &bitbucketv1.Webhook{}
	err := unmarshalAPIResponseValues(r, webhook)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(webhook.ID), nil
}

func createBitbucketServerHook(token, payloadURL string, webhookEvents ...vcsutils.WebhookEvent) *map[string]interface{} {
	return &map[string]interface{}{
		"url":           payloadURL,
		"configuration": map[string]interface{}{"secret": token},
		"events":        getBitbucketServerWebhookEvents(webhookEvents...),
	}
}

// Get varargs of webhook events and return a slice of Bitbucket server webhook events
func getBitbucketServerWebhookEvents(webhookEvents ...vcsutils.WebhookEvent) []string {
	events := make([]string, 0, len(webhookEvents))
	for _, event := range webhookEvents {
		switch event {
		case vcsutils.PrOpened:
			events = append(events, "pr:opened")
		case vcsutils.PrEdited:
			events = append(events, "pr:from_ref_updated")
		case vcsutils.PrMerged:
			events = append(events, "pr:merged")
		case vcsutils.PrRejected:
			events = append(events, "pr:declined", "pr:deleted")
		case vcsutils.Push:
			events = append(events, "repo:refs_changed")
		}
	}
	return events
}

func (client *BitbucketServerClient) mapBitbucketServerCommitToCommitInfo(commit bitbucketv1.Commit,
	owner, repo string) CommitInfo {
	parents := make([]string, len(commit.Parents))
	for i, p := range commit.Parents {
		parents[i] = p.ID
	}
	url := fmt.Sprintf("%s/api/1.0/projects/%s/repos/%s/commits/%s",
		client.vcsInfo.APIEndpoint, owner, repo, commit.ID)
	return CommitInfo{
		Hash:          commit.ID,
		AuthorName:    commit.Author.Name,
		CommitterName: commit.Committer.Name,
		Url:           url,
		Timestamp:     commit.CommitterTimestamp,
		Message:       commit.Message,
		ParentHashes:  parents,
	}
}

func (client *BitbucketServerClient) UploadCodeScanning(ctx context.Context, owner string, repository string, branch string, scanResults string) (string, error) {
	return "", errBitbucketCodeScanningNotSupported
}

type diffPayload struct {
	Diffs []struct {
		Source struct {
			ToString string `mapstructure:"toString"`
		} `mapstructure:"source"`
		Destination struct {
			ToString string `mapstructure:"toString"`
		} `mapstructure:"destination"`
	} `mapstructure:"diffs"`
}

func (client *BitbucketServerClient) GetModifiedFiles(ctx context.Context, owner, repository, refBefore, refAfter string) ([]string, error) {
	err := validateParametersNotBlank(map[string]string{
		"owner":      owner,
		"repository": repository,
		"refBefore":  refBefore,
		"refAfter":   refAfter,
	})
	if err != nil {
		return nil, err
	}

	bitbucketClient, err := client.buildBitbucketClient(ctx)
	if err != nil {
		return nil, err
	}

	params := map[string]interface{}{"contextLines": int32(0), "from": refAfter, "to": refBefore}
	resp, err := bitbucketClient.StreamDiff_37(owner, repository, "", params)
	if err != nil {
		return nil, err
	}

	dst, err := remapFields[diffPayload](resp.Values, "")
	if err != nil {
		return nil, err
	}

	fileNamesSet := datastructures.MakeSet[string]()
	for _, diff := range dst.Diffs {
		fileNamesSet.Add(diff.Source.ToString)
		fileNamesSet.Add(diff.Destination.ToString)
	}
	_ = fileNamesSet.Remove("") // Make sure there are no blank filepath.
	fileNamesList := fileNamesSet.ToSlice()
	sort.Strings(fileNamesList)
	return fileNamesList, nil
}

func getBitbucketServerRepositoryVisibility(public bool) RepositoryVisibility {
	if public {
		return Public
	}
	return Private
}

// BitbucketParseCommitStatuses parse raw response into CommitStatusInfo slice
// The response is the same for BitBucket cloud and server
func BitbucketParseCommitStatuses(rawStatuses interface{}) ([]CommitStatusInfo, error) {
	results := make([]CommitStatusInfo, 0)
	statuses := struct {
		Statuses []struct {
			Title         string `mapstructure:"key"`
			Url           string `mapstructure:"url"`
			State         string `mapstructure:"state"`
			Description   string `mapstructure:"description"`
			Creator       string `mapstructure:"name"`
			LastUpdatedAt string `mapstructure:"updated_on"`
			CreatedAt     string `mapstructure:"created_at"`
		} `mapstructure:"values"`
	}{}
	err := mapstructure.Decode(rawStatuses, &statuses)
	if err != nil {
		return nil, err
	}
	for _, commitStatus := range statuses.Statuses {
		lastUpdatedAt, err := time.Parse(time.RFC3339, commitStatus.LastUpdatedAt)
		if err != nil {
			return nil, err
		}
		createdAt, err := time.Parse(time.RFC3339, commitStatus.LastUpdatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, CommitStatusInfo{
			State:         CommitStatusAsStringToStatus(commitStatus.State),
			Description:   commitStatus.Description,
			DetailsUrl:    commitStatus.Url,
			Creator:       commitStatus.Creator,
			LastUpdatedAt: lastUpdatedAt,
			CreatedAt:     createdAt,
		})
	}
	return results, err
}
