package vcsutils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

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
			// An error occurred
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
			err = safeCopy(targetFile, tarEntryReader)

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
