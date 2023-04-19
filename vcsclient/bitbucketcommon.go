package vcsclient

import (
	"errors"
	"fmt"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/mitchellh/mapstructure"
	"time"
)

var (
	errLabelsNotSupported                          = errors.New("labels are not supported on Bitbucket")
	errBitbucketCodeScanningNotSupported           = errors.New("code scanning is not supported on Bitbucket")
	errBitbucketDownloadFileFromRepoNotSupported   = errors.New("download file from repo is currently not supported on Bitbucket")
	errBitbucketGetRepoEnvironmentInfoNotSupported = errors.New("get repository environment info is currently not supported on Bitbucket")
)

type BitbucketCommitInfo struct {
	Title       string  `mapstructure:"key"`
	Url         string  `mapstructure:"url"`
	State       string  `mapstructure:"state"`
	Creator     string  `mapstructure:"name"`
	Description string  `mapstructure:"description"`
	CreatedOn   string  `mapstructure:"created_on"`
	UpdatedOn   string  `mapstructure:"updated_on"`
	DateAdded   float64 `mapstructure:"DateAdded"`
}

func getBitbucketCommitState(commitState CommitStatus) string {
	switch commitState {
	case Pass:
		return "SUCCESSFUL"
	case Fail, Error:
		return "FAILED"
	case InProgress:
		return "INPROGRESS"
	}
	return ""
}

// bitbucketParseCommitStatuses parse raw response into CommitStatusInfo slice
func bitbucketParseCommitStatuses(rawStatuses interface{}, provider vcsutils.VcsProvider) ([]CommitStatusInfo, error) {
	statuses := struct {
		Statuses []BitbucketCommitInfo `mapstructure:"values"`
	}{}
	if err := mapstructure.Decode(rawStatuses, &statuses); err != nil {
		return nil, err
	}

	var results []CommitStatusInfo
	for i := range statuses.Statuses {
		commitInfo, err := getCommitStatusInfoByBitbucketProvider(&statuses.Statuses[i], provider)
		if err != nil {
			return nil, err
		}
		results = append(results, commitInfo)
	}
	return results, nil
}

func getCommitStatusInfoByBitbucketProvider(commitStatus *BitbucketCommitInfo, provider vcsutils.VcsProvider) (result CommitStatusInfo, err error) {
	switch provider {
	case vcsutils.BitbucketServer:
		return getBitbucketServerCommitStatusInfo(commitStatus), nil
	default:
		return getBitbucketCloudCommitStatusInfo(commitStatus)
	}
}

func getBitbucketServerCommitStatusInfo(commitStatus *BitbucketCommitInfo) CommitStatusInfo {
	// 1. Divide the Unix millisecond timestamp by 1000 to get the Unix time in seconds
	timeInSec := int64(commitStatus.DateAdded) / int64(time.Microsecond)
	// 2. Calculate the nanoseconds value by subtracting the seconds value multiplied by 1000 from the original Unix millisecond timestamp
	//    Finally, multiply the result by 1000000 to get the nanoseconds value
	timeInNanoSec := (int64(commitStatus.DateAdded) - (timeInSec * int64(time.Microsecond))) * int64(time.Millisecond)
	return CommitStatusInfo{
		State:       commitStatusAsStringToStatus(commitStatus.State),
		Description: commitStatus.Description,
		DetailsUrl:  commitStatus.Url,
		Creator:     commitStatus.Title,
		CreatedAt:   time.Unix(timeInSec, timeInNanoSec).UTC(),
	}
}

func getBitbucketCloudCommitStatusInfo(commitStatus *BitbucketCommitInfo) (CommitStatusInfo, error) {
	var createdOn, updatedOn time.Time
	var err error

	if commitStatus.CreatedOn != "" {
		createdOn, err = time.Parse(time.RFC3339, commitStatus.CreatedOn)
		if err != nil {
			return CommitStatusInfo{}, fmt.Errorf("error parsing commit status created_on date: %v", err)
		}
	}
	if commitStatus.UpdatedOn != "" {
		updatedOn, err = time.Parse(time.RFC3339, commitStatus.UpdatedOn)
		if err != nil {
			return CommitStatusInfo{}, fmt.Errorf("error parsing commit status updated_on date: %v", err)
		}
	}

	return CommitStatusInfo{
		State:         commitStatusAsStringToStatus(commitStatus.State),
		Description:   commitStatus.Description,
		DetailsUrl:    commitStatus.Url,
		Creator:       commitStatus.Creator,
		CreatedAt:     createdOn,
		LastUpdatedAt: updatedOn,
	}, nil
}
