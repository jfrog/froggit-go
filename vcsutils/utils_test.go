package vcsutils

import (
	"io"
	"io/ioutil"
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

	fileinfo, err := ioutil.ReadDir(destDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, fileinfo)
	assert.Equal(t, "a", fileinfo[0].Name())
}

func TestUntarRemoveBaseDir(t *testing.T) {
	destDir, tarball := openTarball(t)
	defer tarball.Close()

	err := Untar(destDir, tarball, true)
	assert.NoError(t, err)

	fileinfo, err := ioutil.ReadDir(destDir)
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
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	tarball, err := os.Open(filepath.Join("testdata", "a.tar.gz"))
	assert.NoError(t, err)
	return dir, tarball
}
