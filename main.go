//
// godot
// https://github.com/pmalmgren/godot
//
// Copyright © 2018 Peter Malmgren <me@petermalmgren.com>
// Distributed under the MIT License.
// See README.md for details.
//

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
	"github.com/pmalmgren/godot/conf"
	"github.com/pmalmgren/godot/image"
	"github.com/urfave/cli"
)

const (
	dockerVersion = "1.39"
)

// builds the docker image, this function does a ton of setup with temporary directories
func buildDockerimage(gdc *conf.GoDotConfig) error {
	tmpDir, err := ioutil.TempDir("/tmp/", "godot-build-context")
	if err != nil {
		return fmt.Errorf("Error creating temporary directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Error removing temporary directory: %v", err)
		}
	}()
	// gdc.DockerfileRendered contains the contents of conf/dockerfile_template.go
	dockerfilePath := fmt.Sprintf("%s/Dockerfile", tmpDir)
	dotfileDirectory := fmt.Sprintf("%s/%s", gdc.RepoDirectory, gdc.DotfileDirectory)
	err = ioutil.WriteFile(dockerfilePath, []byte(gdc.DockerfileRendered), 0666)
	if err != nil {
		return fmt.Errorf("Error writing to Docker build context file: %v", err)
	}
	context, err := image.BuildDockerContext(dockerfilePath, dotfileDirectory)
	if err != nil {
		return fmt.Errorf("Error creating Docker build context tarfile: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(filepath.Dir(context)); err != nil {
			log.Printf("Error removing temporary build context: %v", err)
		}
	}()

	// Initialize the Docker CLI client.
	cli, err := client.NewClientWithOpts(client.WithVersion(dockerVersion))
	if err != nil {
		return fmt.Errorf("Error initializing Docker client: %v", err)
	}

	err = image.BuildDockerImage(cli, context, gdc.ImageTag)
	if err != nil {
		return fmt.Errorf("Error building Docker image: %v", err)
	}
	return nil
}

// godot builds and runs the docker image
func godot(u *url.URL) error {
	tmpDir, err := ioutil.TempDir("", "/tmp")
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Error removing temporary Git repo: %v", err)
		}
	}()
	if err != nil {
		return fmt.Errorf("Error creating temporary directory: %v", err)
	}
	repo := conf.Repository{Remote: u, RepoDirectory: tmpDir}
	if err := repo.Pull(); err != nil {
		return fmt.Errorf("Error reading from Git repository: %v", err)
	}

	gdc, err := conf.ConfigFromReadme(&repo)
	if err != nil {
		return fmt.Errorf("Error parsing README.md configuration: %v", err)
	}

	err = buildDockerimage(gdc)
	if err != nil {
		return fmt.Errorf("Error building Docker Image: %v", err)
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "godot"
	app.Usage = "godot build your-repo"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name:    "build",
			Aliases: []string{"b"},
			Action: func(ctx *cli.Context) error {
				repoStr := ctx.Args().Get(len(ctx.Args()) - 1)
				u, err := url.Parse(repoStr)
				if err != nil {
					return fmt.Errorf("Error parsing repository: %v", err)
				}
				if err := godot(u); err != nil {
					return fmt.Errorf("Error: %v", err)
				}
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
