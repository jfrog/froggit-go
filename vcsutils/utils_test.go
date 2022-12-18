package vcsutils

import (
	"github.com/go-git/go-git/v5"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUntar(t *testing.T) {
	destDir, tarball := openTarball(t)
	defer tarball.Close()

	err := Untar(destDir, tarball, false)
	assert.NoError(t, err)

	fileinfo, err := os.ReadDir(destDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, fileinfo)
	assert.Equal(t, "a", fileinfo[0].Name())
}

func TestUntarRemoveBaseDir(t *testing.T) {
	destDir, tarball := openTarball(t)
	defer tarball.Close()

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
	defer os.RemoveAll(dir)

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
	defer assert.NoError(t, os.RemoveAll(destDir))
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

<<<<<<< HEAD
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
		StatusCode: 200,
	}
	assert.NoError(t, CheckResponseStatusWithBody(resp, expectedStatusCode))
=======
func TestCreateDotGitFolderWithRemote(t *testing.T) {
	dir1, err := os.MkdirTemp("", "tmp")
	assert.NoError(t, err)
	defer os.RemoveAll(dir1)
	err = CreateDotGitFolderWithRemote(dir1, "origin", "fakeurl")
	assert.NoError(t, err)
	repo, err := git.PlainOpen(filepath.Join(dir1, ".git"))
	assert.NoError(t, err)
	remote, err := repo.Remote("origin")
	assert.NoError(t, err)
	assert.NotNil(t, remote)
	assert.Contains(t, remote.Config().URLs, "fakeurl")
	// Return error if .git already exist
	assert.Error(t, CreateDotGitFolderWithRemote(dir1, "origin", "fakeurl"))
}

func TestCreateDotGitFolderWithoutRemote(t *testing.T) {
	// Return error if remote name is empty
	dir2, err := os.MkdirTemp("", "tmp")
	assert.NoError(t, err)
	defer os.RemoveAll(dir2)
	assert.Error(t, CreateDotGitFolderWithRemote(dir2, "", "fakeurl"))
>>>>>>> upstream/master
}
