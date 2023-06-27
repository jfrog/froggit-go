package vcsutils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const RemoteName = "origin"

// CreateToken create a random UUID
func CreateToken() string {
	return uuid.New().String()
}

// Untar a file to the given destination
// destDir             - Destination folder
// reader              - Reader for the tar.gz file
// shouldRemoveBaseDir - True if should remove the base directory
func Untar(destDir string, reader io.Reader, shouldRemoveBaseDir bool) (err error) {
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer func() {
		e := gzr.Close()
		if err == nil {
			err = e
		}
	}()

	if err := makeDirIfMissing(destDir); err != nil {
		return err
	}

	var header *tar.Header
	for tarEntryReader := tar.NewReader(gzr); err != io.EOF; header, err = tarEntryReader.Next() {
		if err != nil {
			return err
		}

		if header == nil {
			// Header is missing, skip
			continue
		}

		// Remove the root directory of the repository if needed
		filePath := header.Name
		if shouldRemoveBaseDir {
			filePath = removeBaseDir(filePath)
		}
		if filePath == "" {
			continue
		}

		// The target location where the dir/file should be created
		target, err := sanitizeExtractionPath(filePath, destDir)
		if err != nil {
			return err
		}

		// Check the file type
		switch header.Typeflag {

		// If its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0750); err != nil {
					return err
				}
			}

		// If it's a file create it
		case tar.TypeReg:
			targetFile, err := os.OpenFile(filepath.Clean(target), os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// Copy file contents
			if err = safeCopy(targetFile, tarEntryReader); err != nil {
				return err
			}

			// Manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			if err := targetFile.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func makeDirIfMissing(destDir string) error {
	var err error
	if _, err = os.Stat(destDir); os.IsNotExist(err) {
		err = os.MkdirAll(destDir, 0700)
	}
	return err
}

// Remove the left component of the relative path
func removeBaseDir(relativePath string) string {
	parts := strings.Split(filepath.Clean(relativePath), string(os.PathSeparator))
	if len(parts) < 2 {
		return ""
	}
	return filepath.Join(parts[1:]...)
}

func sanitizeExtractionPath(filePath string, destination string) (string, error) {
	target := filepath.Join(destination, filepath.Clean(filePath))
	if !strings.HasPrefix(target, filepath.Clean(destination)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: illegal file path", filePath)
	}
	return target, nil
}

func safeCopy(targetFile *os.File, reader io.Reader) error {
	for {
		_, err := io.CopyN(targetFile, reader, 1024)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// DiscardResponseBody prepare http response body for closing
func DiscardResponseBody(resp *http.Response) error {
	if resp != nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	return nil
}

// GetZeroValue returns the zero value of type T
func GetZeroValue[T any]() T {
	return *new(T)
}

// DefaultIfNotNil checks:
// 1. If the pointer is nil, return the zero value of the type
// 2. If the pointer isn't nil, return the value of the pointer.
func DefaultIfNotNil[T any](val *T) T {
	if val == nil {
		return GetZeroValue[T]()
	}
	return *val
}

// PointerOf returns pointer to the provided value if it is not nil.
func PointerOf[T any](v T) *T {
	return &v
}

// AddBranchPrefix adds a branchPrefix to a branch name if it is not already present.
func AddBranchPrefix(branch string) string {
	if !strings.HasPrefix(branch, branchPrefix) {
		branch = fmt.Sprintf("%s%s", branchPrefix, branch)
	}
	return branch
}

// Unzip a file to dest path
func Unzip(zipFileContent []byte, destinationToUnzip string) (err error) {
	zf, err := zip.NewReader(bytes.NewReader(zipFileContent), int64(len(zipFileContent)))
	if err != nil {
		return err
	}
	// Get the absolute destination path
	destinationToUnzip, err = filepath.Abs(destinationToUnzip)
	if err != nil {
		return err
	}

	// Iterate over zip files inside the archive and unzip each of them
	for _, f := range zf.File {
		err = unzipFile(f, destinationToUnzip)
		if err != nil {
			return err
		}
	}

	return nil
}

func unzipFile(f *zip.File, destination string) (err error) {
	// Check if file paths are not vulnerable to Zip Slip
	fullFilePath, err := sanitizeExtractionPath(f.Name, destination)
	if err != nil {
		return err
	}
	// Create directory tree
	if f.FileInfo().IsDir() {
		if e := os.MkdirAll(fullFilePath, 0700); err == nil {
			return e
		}
		return nil
	} else if err = os.MkdirAll(filepath.Dir(fullFilePath), 0700); err != nil {
		return err
	}

	// Create a destination file for unzipped content
	destinationFile, err := os.OpenFile(filepath.Clean(fullFilePath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if e := destinationFile.Close(); err == nil {
			err = e
			return
		}
	}()
	// Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		if e := zippedFile.Close(); err == nil {
			err = e
			return
		}
	}()
	return safeCopy(destinationFile, zippedFile)
}

func CheckResponseStatusWithBody(resp *http.Response, expectedStatusCodes ...int) error {
	for _, statusCode := range expectedStatusCodes {
		if statusCode == resp.StatusCode {
			return nil
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return GenerateResponseError(resp.Status, generateErrorString(body))
}

func GenerateResponseError(status, body string) error {
	responseErrString := "server response: " + status
	if body != "" {
		responseErrString = responseErrString + "\n" + body
	}
	return fmt.Errorf(responseErrString)
}

func generateErrorString(bodyArray []byte) string {
	var content bytes.Buffer
	if len(bodyArray) > 0 {
		if err := json.Indent(&content, bodyArray, "", "  "); err != nil {
			return string(bodyArray)
		}
		return content.String()
	}
	return ""
}

// CreateDotGitFolderWithRemote creates a .git folder inside path with remote details of remoteName and remoteUrl
func CreateDotGitFolderWithRemote(path, remoteName, remoteUrl string) error {
	repo, err := git.PlainInit(path, false)
	if err == git.ErrRepositoryAlreadyExists {
		// If the .git folder already exists, we can skip this function
		return nil
	}
	if err != nil {
		return err
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{remoteUrl},
	})
	return err
}

func GetGenericGitRemoteUrl(apiEndpoint, owner, repo string) string {
	return fmt.Sprintf("%s/%s/%s.git", strings.TrimSuffix(apiEndpoint, "/"), owner, repo)
}

// RemapFields creates an instance of the T type and copies data from src parameter to it
// by mapping fields based on the tags with tagName (if not provided 'mapstructure' tag is used)
// using 'mapstructure' library.
func RemapFields[T any](src any, tagName string) (T, error) {
	var dst T
	if changeDecoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:    tagName,
		Result:     &dst,
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}); err != nil {
		return dst, err
	} else if err := changeDecoder.Decode(src); err != nil {
		return dst, err
	}
	return dst, nil
}

func MapPullRequestState(state *PullRequestState) *string {
	var stateStringValue string
	switch *state {
	case Open:
		stateStringValue = "open"
	case Closed:
		stateStringValue = "closed"
	default:
		return nil
	}
	return &stateStringValue
}

func ValidateParametersNotBlank(paramNameValueMap map[string]string) error {
	var errorMessages []string
	for k, v := range paramNameValueMap {
		if strings.TrimSpace(v) == "" {
			errorMessages = append(errorMessages, fmt.Sprintf("required parameter '%s' is missing", k))
		}
	}
	if len(errorMessages) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errorMessages, ", "))
	}
	return nil
}

// commitStatusAsStringToStatus maps status as string to CommitStatus
// Handles all the different statuses for every VCS provider
func CommitStatusAsStringToStatus(rawStatus string) CommitStatus {
	switch strings.ToLower(rawStatus) {
	case "success", "succeeded", "successful":
		return Pass
	case "fail", "failure", "failed":
		return Fail
	case "pending", "inprogress":
		return InProgress
	default:
		return Error
	}
}

func ExtractTimeWithFallback(timeObject *time.Time) time.Time {
	if timeObject == nil {
		return time.Time{}
	}
	return timeObject.UTC()
}
