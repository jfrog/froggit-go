package vcsclient

import (
	"errors"
	"github.com/mitchellh/mapstructure"
	"time"
)

var errLabelsNotSupported = errors.New("labels are not supported on Bitbucket")
var errBitbucketCodeScanningNotSupported = errors.New("code scanning is not supported on Bitbucket")

var errBitbucketDownloadFileFromRepoNotSupported = errors.New("download file from repo is currently not supported on Bitbucket")
var errBitbucketGetRepoEnvironmentInfoNotSupported = errors.New("get repository environment info is currently not supported on Bitbucket")

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
// The response is the same for BitBucket cloud and server
func bitbucketParseCommitStatuses(rawStatuses interface{}) ([]CommitStatusInfo, error) {
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
