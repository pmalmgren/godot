//
// godot
// https://github.com/pmalmgren/godot
//
// Copyright © 2018 Peter Malmgren <me@petermalmgren.com>
// Distributed under the MIT License.
// See README.md for details.
//
package image

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
)

func getBuildContext(t *testing.T) string {
	tmpDir, err := ioutil.TempDir("/tmp", "")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Error removing temporary directory %s: %v", tmpDir, err)
		}
	}()
	dockerPath := fmt.Sprintf("%s/Dockerfile", tmpDir)
	testDirPath := fmt.Sprintf("%s/dotfiles", tmpDir)
	testFilePath := fmt.Sprintf("%s/test.txt", testDirPath)
	if err := ioutil.WriteFile(dockerPath, []byte("foo"), 0777); err != nil {
		t.Fatalf("Error writing temporary Dockerfile: %v", err)
	}
	if err := os.Mkdir(testDirPath, 0777); err != nil {
		t.Fatalf("Error writing temporary directory: %v", err)
	}
	if err := ioutil.WriteFile(testFilePath, []byte("bar"), 0644); err != nil {
		t.Fatalf("Error writing test file: %v", err)
	}

	dockerContext, err := BuildDockerContext(dockerPath, testDirPath)
	if err != nil {
		t.Fatalf("Error creating docker build context: %v", err)
	}

	return dockerContext
}

func TestBuildDockerContext(t *testing.T) {
	dockerContext := getBuildContext(t)
	file, err := os.Open(dockerContext)
	if err != nil {
		t.Fatalf("Error opening build context: %v", err)
	}
	defer func() {
		if err := os.Remove(dockerContext); err != nil {
			t.Logf("Error removing temporary build context: %v", err)
		}
		if err := os.RemoveAll(filepath.Dir(dockerContext)); err != nil {
			t.Logf("Error removing temporary build context: %v", err)
		}
	}()
	tr := tar.NewReader(bufio.NewReader(file))
	actual := make(map[string]string)
	for {
		buf := make([]byte, 12)
		hdrf, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Fatal error reading Docker Context: %v", err)
		}
		if _, err := tr.Read(buf); err != nil && err != io.EOF {
			t.Fatalf("Fatal error reading from tarfile: %v", err)
		}
		actual[hdrf.Name] = string(bytes.Trim(buf, "\x00"))
	}
	expected := map[string]string{"Dockerfile": "foo", "dotfiles/test.txt": "bar"}
	for k, v := range expected {
		av, ok := actual[k]
		if !ok {
			t.Errorf("Dockerfile missing file: %s", k)
		}
		if av != v {
			t.Errorf("Dockerfile contents of file %s invalid: %v != %v", k, []byte(av), []byte(v))
		}
	}
}

type MockDockerClient struct {
	Response   types.ImageBuildResponse
	Error      error
	Dockerfile string
	Tags       []string
	t          *testing.T
}

func (mdc *MockDockerClient) ImageBuild(ctx context.Context, buf io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	mdc.Dockerfile = string(options.Dockerfile)
	mdc.Tags = options.Tags

	return mdc.Response, mdc.Error
}

func TestBuildDockerImage(t *testing.T) {
	response := types.ImageBuildResponse{Body: ioutil.NopCloser(bytes.NewReader([]byte("** TEST **")))}
	mdc := &MockDockerClient{Error: nil, Response: response, t: t, Tags: []string{}}
	dockerContext := getBuildContext(t)

	err := BuildDockerImage(mdc, dockerContext, "test")
	if err != nil {
		t.Fatalf("BuildDockerImage unexpected error: %v", err)
	}

	if len(mdc.Tags) != 1 || mdc.Tags[0] != "test" {
		t.Fatalf("Docker client called with unexpected tags: %+v", mdc.Tags)
	}
	if mdc.Dockerfile != "Dockerfile" {
		t.Fatalf("Docker client called with unexpected Dockerfile: %+v", mdc.Dockerfile)
	}
}
