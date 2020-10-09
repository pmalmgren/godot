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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/jhoonb/archivex"
)

type imagebuilder interface {
	ImageBuild(context.Context, io.Reader, types.ImageBuildOptions) (types.ImageBuildResponse, error)
}

type dockerCliOutput struct {
	Stream string `json:"stream"`
}

// BuildDockerImage builds a Docker image from a directory, specified on contextPath
func BuildDockerImage(cli imagebuilder, contextPath string, tag string) error {
	dockerBuildContext, err := os.Open(contextPath)
	if err != nil {
		return fmt.Errorf("Error opening build context tarfile: %v", err)
	}
	options := types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Tags:           []string{tag},
		Dockerfile:     "Dockerfile",
	}
	buildResponse, err := cli.ImageBuild(context.Background(), dockerBuildContext, options)
	if err != nil {
		return fmt.Errorf("Error building Docker image: %v", err)
	}
	defer func() {
		if err := buildResponse.Body.Close(); err != nil {
			log.Printf("Error closing Docker build response body: %v", err)
		}
	}()

	log.Printf("Building Docker image from build context %s", contextPath)

	reader := bufio.NewReader(buildResponse.Body)
	for {
		line, err := reader.ReadBytes('\r')
		if err != nil && err != io.EOF {
			return fmt.Errorf("Error reading from Docker: %v", err)
		}

		line = bytes.TrimSpace(line)
		var output dockerCliOutput
		json.Unmarshal(line, &output)

		fmt.Printf("%s", output.Stream)

		if err != nil {
			break
		}
	}

	return nil
}

// isolateDockerfile moves a Dockerfile to its own temporary directory
func isolateDockerfile(dockerfilePath string) (string, error) {
	dockerfileContents, err := ioutil.ReadFile(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("Error reading from Dockerfile: %v", err)
	}

	tmpDir, err := ioutil.TempDir("/tmp/", "dockerfile-directory")
	if err != nil {
		return "", fmt.Errorf("Error creating temporary directory: %v", err)
	}

	newDockerfilePath := fmt.Sprintf("%s/Dockerfile", tmpDir)
	err = ioutil.WriteFile(newDockerfilePath, []byte(dockerfileContents), 0666)
	if err != nil {
		return "", fmt.Errorf("Error writing to Docker build context file: %v", err)
	}

	return tmpDir, nil
}

// BuildDockerContext adds directories and a Dockerfile to a tarball.
// The caller is responsible for cleanup.
func BuildDockerContext(dockerfilePath string, dirs ...string) (string, error) {
	// move the Dockerfile to its own path
	newDockerfilePath, err := isolateDockerfile(dockerfilePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(newDockerfilePath); err != nil {
			log.Printf("Error removing Dockerfile path: %v", err)
		}
	}()

	tar := new(archivex.TarFile)
	tmpDir, err := ioutil.TempDir("/tmp", "")
	if err != nil {
		return "", fmt.Errorf("Error creating a temporary directory: %v", err)
	}
	tarPath := fmt.Sprintf("%s/buildcontext.tar", tmpDir)
	if err := tar.Create(tarPath); err != nil {
		log.Printf("Error creating Docker build context tarfile: %v", err)
	}
	if err := tar.AddAll(newDockerfilePath, false); err != nil {
		return "", fmt.Errorf("Error adding Dockerfile to build context: %v", err)
	}

	for _, dir := range dirs {
		if err := tar.AddAll(dir, true); err != nil {
			return "", fmt.Errorf("Error adding directory %s to build context: %v", dir, err)
		}
	}

	if err := tar.Close(); err != nil {
		return "", fmt.Errorf("Error closing Docker build context file: %v", err)
	}

	return tarPath, nil
}
