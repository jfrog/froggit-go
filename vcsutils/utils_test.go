package vcsutils

import (
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
	defer os.RemoveAll(destDir)

	err = Unzip(filepath.Join("testdata", "hello_world.zip"), destDir)
	assert.NoError(t, err)

	fileinfo, err := os.ReadDir(destDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, fileinfo)
	assert.Equal(t, "README.md", fileinfo[0].Name())
}
