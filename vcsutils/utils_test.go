package vcsutils

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUntar(t *testing.T) {
	destDir, tarball := openTarball(t)
	defer func() {
		assert.NoError(t, tarball.Close())
	}()

	err := Untar(destDir, tarball, false)
	assert.NoError(t, err)

	fileinfo, err := os.ReadDir(destDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, fileinfo)
	assert.Equal(t, "a", fileinfo[0].Name())
}

func TestUntarRemoveBaseDir(t *testing.T) {
	destDir, tarball := openTarball(t)
	defer func() {
		assert.NoError(t, tarball.Close())
	}()

	err := Untar(destDir, tarball, true)
	assert.NoError(t, err)

	fileinfo, err := os.ReadDir(destDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, fileinfo)
	assert.Equal(t, "b", fileinfo[0].Name())
}

func TestUntarError(t *testing.T) {
	err := Untar("", io.MultiReader(), false)
	assert.Error(t, err)
}

func TestCreateToken(t *testing.T) {
	assert.NotEmpty(t, CreateToken())
}

func TestSanitizeExtractionPath(t *testing.T) {
	_, err := sanitizeExtractionPath("../a", "a")
	assert.EqualError(t, err, "../a: illegal file path")
}

func TestSafeCopyError(t *testing.T) {
	err := safeCopy(nil, nil)
	assert.Error(t, err)
}

type removeBaseDirDataProvider struct {
	relativePath string
	expectedPath string
}

var removeBaseDirWindowsProvider = []removeBaseDirDataProvider{
	{"a", ""},
	{"a\\b", "b"},
	{"a\\b\\c", "b\\c"},
	{"/", ""},
	{" ", ""},
}

var removeBaseDirUnixProvider = []removeBaseDirDataProvider{
	{"a", ""},
	{"b/c", "c"},
	{"a/b/c", "b/c"},
	{"/", ""},
	{" ", ""},
}

func TestRemoveBaseDir(t *testing.T) {
	var testCases []removeBaseDirDataProvider
	if os.PathSeparator == '/' {
		testCases = removeBaseDirUnixProvider
	} else {
		testCases = removeBaseDirWindowsProvider
	}
	for _, testCase := range testCases {
		t.Run(testCase.relativePath, func(t *testing.T) {
			assert.Equal(t, testCase.expectedPath, removeBaseDir(testCase.relativePath))
		})
	}
}

func TestDiscardResponseBody(t *testing.T) {
	assert.NoError(t, DiscardResponseBody(nil))
	assert.NoError(t, DiscardResponseBody(&http.Response{Body: io.NopCloser(io.MultiReader())}))
}

func openTarball(t *testing.T) (string, *os.File) {
	dir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, RemoveTempDir(dir))
	}()

	tarball, err := os.Open(filepath.Join("testdata", "a.tar.gz"))
	assert.NoError(t, err)
	return dir, tarball
}

func TestDefaultIfNotNil(t *testing.T) {
	str := "Hello world"
	assert.Equal(t, "Hello world", DefaultIfNotNil(&str))
	assert.Equal(t, "", DefaultIfNotNil(new(string)))
	num := 3
	assert.Equal(t, 3, DefaultIfNotNil(&num))
	assert.Equal(t, 0, DefaultIfNotNil(new(int)))
	boolVal := true
	assert.Equal(t, true, DefaultIfNotNil(&boolVal))
	assert.Equal(t, false, DefaultIfNotNil(new(bool)))
	arr := []int{3, 4}
	assert.Equal(t, []int{3, 4}, DefaultIfNotNil(&arr))
	assert.Equal(t, []int(nil), DefaultIfNotNil(new([]int)))
}

func TestUnzip(t *testing.T) {
	destDir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer assert.NoError(t, RemoveTempDir(destDir))
	zipFileContent, err := os.ReadFile(filepath.Join("testdata", "hello_world.zip"))
	assert.NoError(t, err)
	err = Unzip(zipFileContent, destDir)
	assert.NoError(t, err)

	fileinfo, err := os.ReadDir(destDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, fileinfo)
	assert.Equal(t, "README.md", fileinfo[0].Name())
}

func TestAddBranchPrefix(t *testing.T) {
	branch := "sampleBranch"
	branchWithPrefix := AddBranchPrefix(branch)
	assert.Equal(t, branchWithPrefix, "refs/heads/sampleBranch")
	branchWithPrefix = AddBranchPrefix(branchWithPrefix)
	assert.Equal(t, branchWithPrefix, "refs/heads/sampleBranch")
}

func TestGetZeroValue(t *testing.T) {
	assert.Equal(t, 0, GetZeroValue[int]())
	assert.Equal(t, "", GetZeroValue[string]())
	assert.Equal(t, 0.0, GetZeroValue[float64]())
}

func TestGenerateResponseError(t *testing.T) {
	status := "404"
	emptyBodyErr := GenerateResponseError(status, "")
	assert.Error(t, emptyBodyErr)
	assert.Equal(t, "server response: 404", emptyBodyErr.Error())
	err := GenerateResponseError(status, "error")
	assert.Error(t, err)
	assert.Equal(t, "server response: 404\nerror", err.Error())
}

func TestCheckResponseStatusWithBody(t *testing.T) {
	expectedStatusCode := 200
	resp := &http.Response{
		Status:     "200",
		StatusCode: http.StatusOK,
	}
	assert.NoError(t, CheckResponseStatusWithBody(resp, expectedStatusCode))
}

func TestCreateDotGitFolderWithRemote(t *testing.T) {
	dir1, err := os.MkdirTemp("", "tmp")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, RemoveTempDir(dir1))
	}()
	err = CreateDotGitFolderWithRemote(dir1, "origin", "fakeurl")
	assert.NoError(t, err)
	repo, err := git.PlainOpen(filepath.Join(dir1, ".git"))
	assert.NoError(t, err)
	remote, err := repo.Remote("origin")
	assert.NoError(t, err)
	assert.NotNil(t, remote)
	assert.Contains(t, remote.Config().URLs, "fakeurl")
	// Return no err if .git already exist
	assert.NoError(t, CreateDotGitFolderWithRemote(dir1, "origin", "fakeurl"))
}

func TestCreateDotGitFolderWithoutRemote(t *testing.T) {
	// Return error if remote name is empty
	dir2, err := os.MkdirTemp("", "tmp")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, RemoveTempDir(dir2))
	}()
	assert.Error(t, CreateDotGitFolderWithRemote(dir2, "", "fakeurl"))
}

func TestPointerOf(t *testing.T) {
	assert.Equal(t, 5, *PointerOf(5))
	assert.Equal(t, "some", *PointerOf("some"))
}

func TestRemapFields(t *testing.T) {
	type destination struct {
		Name      string    `some:"n_ame"`
		Birthdate time.Time `some:"B_day"`
		High      int       `json:"high"`
	}

	date := time.Date(2020, 10, 9, 8, 7, 6, 0, time.UTC)
	src := map[string]any{"n_ame": "John", "B_day": date.Format(time.RFC3339)}
	result, err := RemapFields[destination](src, "some")
	assert.NoError(t, err)
	assert.Equal(t, destination{Name: "John", Birthdate: date}, result)
}

func TestMapPullRequestState(t *testing.T) {
	testCases := []struct {
		state       PullRequestState
		expected    string
		gitProvider VcsProvider
	}{
		{state: Open, expected: "open", gitProvider: GitHub},
		{state: Closed, expected: "closed", gitProvider: GitHub},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s-%s", tc.gitProvider.String(), tc.state), func(t *testing.T) {
			assert.Equal(t, tc.expected, *MapPullRequestState(&tc.state))
		})
	}
}

func TestRemoveDirContents(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	defer func() {
		assert.NoError(t, os.Remove(tmpDir))
	}()

	// Create some test files and directories inside the temporary directory
	testFiles := []string{"file1.txt", "file2.txt"}
	testDirs := []string{"dir1", "dir2"}

	for _, fileName := range testFiles {
		filePath := filepath.Join(tmpDir, fileName)
		_, err := os.Create(filePath)
		assert.NoError(t, err)
	}

	for _, dirName := range testDirs {
		dirPath := filepath.Join(tmpDir, dirName)
		err := os.Mkdir(dirPath, os.ModeDir)
		assert.NoError(t, err)
	}

	// Test the RemoveDirContents function
	err := RemoveDirContents(tmpDir)
	assert.NoError(t, err)

	// Check if the temporary directory is empty after removal
	entries, err := os.ReadDir(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

func TestGetPullRequestFilePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"file.txt", "/file.txt"},
		{"/path/to/file.txt", "/path/to/file.txt"},
		{"dir/file.txt", "/dir/file.txt"},
	}

	for _, test := range tests {
		result := GetPullRequestFilePath(test.input)
		assert.Equal(t, test.expected, result)
	}
}
