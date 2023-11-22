package vcsutils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/exp/slices"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	RemoteName = "origin"
)

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
		return
	}
	defer func() { err = errors.Join(err, gzr.Close()) }()

	if err = makeDirIfMissing(destDir); err != nil {
		return
	}

	var header *tar.Header
	var readerErr error
	for tarEntryReader := tar.NewReader(gzr); readerErr != io.EOF; header, readerErr = tarEntryReader.Next() {
		if readerErr != nil {
			return
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
		var target string
		target, err = sanitizeExtractionPath(filePath, destDir)
		if err != nil {
			return
		}

		// Check the file type
		switch header.Typeflag {

		// If it's a dir, and it doesn't exist create it
		case tar.TypeDir:
			err = makeDirIfMissing(target)
			if err != nil {
				return
			}

		// If it's a file create it
		case tar.TypeReg:
			var targetFile *os.File
			targetFile, err = os.OpenFile(filepath.Clean(target), os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return
			}

			// Copy file contents
			if err = safeCopy(targetFile, tarEntryReader); err != nil {
				return
			}

			// Manually close here after each file operation; deferring would cause each file close
			// to wait until all operations have completed.
			if err = targetFile.Close(); err != nil {
				return
			}
		}
	}
	return
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

func isZeroValue[T comparable](val T) bool {
	zeroValue := GetZeroValue[T]()
	return zeroValue == val
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

func GetNilIfZeroVal[T comparable](val T) *T {
	if isZeroValue(val) {
		return nil
	}
	return &val
}

// PointerOf returns pointer to the provided value if it is not nil.
func PointerOf[T any](v T) *T {
	return &v
}

// AddBranchPrefix adds a branchPrefix to a branch name if it is not already present.
func AddBranchPrefix(branch string) string {
	if branch != "" && !strings.HasPrefix(branch, branchPrefix) {
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
		err = errors.Join(err, destinationFile.Close())
	}()
	// Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, zippedFile.Close())
	}()
	return safeCopy(destinationFile, zippedFile)
}

func CheckResponseStatusWithBody(resp *http.Response, expectedStatusCodes ...int) error {
	if resp == nil {
		return errors.New("received an empty response")
	}

	if slices.Contains(expectedStatusCodes, resp.StatusCode) {
		return nil
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
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
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

func RemoveTempDir(dirPath string) error {
	if err := os.RemoveAll(dirPath); err == nil {
		return nil
	}
	// Sometimes removing the directory fails (in Windows) because it's locked by another process.
	// That's a known issue, but its cause is unknown (golang.org/issue/30789).
	// In this case, we'll only remove the contents of the directory, and let CleanOldDirs() remove the directory itself at a later time.
	return RemoveDirContents(dirPath)
}

// RemoveDirContents removes the contents of the directory, without removing the directory itself.
// If it encounters an error before removing all the files, it stops and returns that error.
func RemoveDirContents(dirPath string) (err error) {
	d, err := os.Open(dirPath)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, d.Close())
	}()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dirPath, name))
		if err != nil {
			return
		}
	}
	return
}

func GetPullRequestFilePath(filePath string) string {
	if filePath == "" {
		return ""
	}
	return fmt.Sprintf("/%s", strings.TrimPrefix(filePath, "/"))
}
