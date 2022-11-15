package vcsutils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
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
	target := filepath.Join(destination, filePath)
	if !strings.HasPrefix(target, filepath.Clean(destination)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: illegal file path", filePath)
	}
	return target, nil
}

func safeCopy(targetFile *os.File, reader *tar.Reader) error {
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
		_, err := io.Copy(ioutil.Discard, resp.Body)
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

// Unzip a file in src to dest path
func Unzip(source, destination string) (err error) {
	// Open the zip file
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer func() {
		e := reader.Close()
		if err == nil {
			err = e
		}
	}()

	// Get the absolute destination path
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}

	// Iterate over zip files inside the archive and unzip each of them
	for _, f := range reader.File {
		err := unzipFile(f, destination)
		if err != nil {
			return err
		}
	}

	return nil
}

func unzipFile(f *zip.File, destination string) error {
	fullFilePath := filepath.Join(destination, filepath.Clean(f.Name))
	// Check if file paths are not vulnerable to Zip Slip
	if !strings.HasPrefix(fullFilePath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", fullFilePath)
	}

	// Create directory tree
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(fullFilePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(fullFilePath), os.ModePerm); err != nil {
		return err
	}

	// Create a destination file for unzipped content
	destinationFile, err := os.OpenFile(filepath.Clean(fullFilePath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if err := destinationFile.Close(); err != nil {
			return
		}
	}()

	// Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		if err := zippedFile.Close(); err != nil {
			return
		}
	}()

	for {
		if _, err := io.CopyN(destinationFile, zippedFile, 1024); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	return nil
}
