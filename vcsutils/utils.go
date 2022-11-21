package vcsutils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// CreateToken create a random UUID
func CreateToken() string {
	return uuid.New().String()
}

// Untar a file to the given destination
// destDir             - Destination folder
// reader              - Reader for the tar.gz file
// shouldRemoveBaseDir - True if should remove the base directory
func Untar(destDir string, reader io.Reader, shouldRemoveBaseDir bool) error {
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gzr.Close()

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

func safeCopy(targetFile *os.File, v io.Reader) error {
	for {
		_, err := io.CopyN(targetFile, v, 1024)
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
