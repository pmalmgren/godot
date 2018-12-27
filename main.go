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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/jhoonb/archivex"
	"github.com/urfave/cli"
	git "gopkg.in/src-d/go-git.v4"
	yaml "gopkg.in/yaml.v2"
)

const (
	confHeader        = "## godot configuration"
	confBoundaryToken = "```"
)

// GoDotConfig contains the relevant configuration to pass to the Dockerfile template
type GoDotConfig struct {
	Username           string   `yaml:"username"`
	DotfileDirectory   string   `yaml:"dotfile-directory"`
	Packages           []string `yaml:"packages"`
	SystemSetup        []string `yaml:"system-setup"`
	UserSetup          []string `yaml:"user-setup"`
	EntryPoint         string   "yaml:`entrypoint`"
	ImageTag           string
	OutputDirectory    string
	RepoDirectory      string
	DockerfileRendered string
}

// fetchReadme grabs the README.md from the repository
func fetchReadme(u *url.URL) (string, string, error) {
	localRepoDirectory, err := ioutil.TempDir("/tmp", u.EscapedPath())
	if err != nil {
		return "", "", fmt.Errorf("Error creating temporary directory: %v", err)
	}

	_, err = git.PlainClone(localRepoDirectory, false, &git.CloneOptions{
		URL:   u.String(),
		Depth: 1,
	})

	if err != nil && err != git.ErrRepositoryAlreadyExists {
		return "", "", fmt.Errorf("Error cloning repository: %v", err)
	}

	readmePath := fmt.Sprintf("%s/README.md", localRepoDirectory)

	if _, err := os.Stat(readmePath); err != nil {
		return "", "", fmt.Errorf("README.md does not exist at %s", readmePath)
	}

	return readmePath, localRepoDirectory, nil
}

// parseConfig parses out the godot configuration from a README file
func parseConfig(readmePath string, repoPath string) (*GoDotConfig, error) {
	f, err := os.OpenFile(readmePath, os.O_RDONLY, os.ModePerm)

	if err != nil {
		return nil, fmt.Errorf("Error opening README.md: %v", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing README.md: %v", err)
		}
	}()

	sc := bufio.NewScanner(f)
	var rawConfig string
	var sawHeader bool
	var insideConfig bool
	for sc.Scan() {
		token := strings.TrimSuffix(sc.Text(), "\n")

		if !sawHeader {
			sawHeader = token == confHeader
			continue
		}

		if !insideConfig {
			insideConfig = token == confBoundaryToken
			continue
		}

		if insideConfig {
			if token == confBoundaryToken {
				break
			}
			rawConfig += token
			rawConfig += "\n"
		}
	}

	var gdc GoDotConfig
	if err := yaml.Unmarshal([]byte(rawConfig), &gdc); err != nil {
		return nil, fmt.Errorf("Error reading repository configuration: %v", err)
	}
	gdc.RepoDirectory = repoPath
	return &gdc, nil
}

// validates & applies the configuration to the template
func buildDockerfile(gdc *GoDotConfig, repoPath string) (string, error) {
	t, err := template.ParseFiles("Dockerfile.tmpl")
	if err != nil {
		return "", fmt.Errorf("Error parsing template file: %v", err)
	}

	f, err := os.Create("Dockerfile.godot")
	if err != nil {
		return "", fmt.Errorf("Error creating rendered Dockerfile.godot: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing Dockerfile")
		}
	}()
	var tpl bytes.Buffer
	err = t.Execute(&tpl, gdc)

	if err != nil {
		return "", fmt.Errorf("Error rendering Dockerfile.tmpl: %v", err)
	}
	renderedTemplate := tpl.String()

	if _, err := f.Write([]byte(renderedTemplate)); err != nil {
		return "", fmt.Errorf("Error writing rendered template to file: %v", err)
	}

	return renderedTemplate, nil
}

// optionally, build the docker image
func buildDockerimage(gdc *GoDotConfig) error {
	tmpDir, err := ioutil.TempDir("/tmp/", "godot-build-context")
	if err != nil {
		return fmt.Errorf("Error creating temporary directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Error removing temporary directory: %v", err)
		}
	}()
	err = ioutil.WriteFile(fmt.Sprintf("%s/Dockerfile", tmpDir), []byte(gdc.DockerfileRendered), 0666)
	if err != nil {
		return fmt.Errorf("Error writing to Docker build context file: %v", err)
	}

	tar := new(archivex.TarFile)
	var closed bool
	if err := tar.Create("/tmp/godot-buildcontext.tar"); err != nil {
		log.Printf("Error creating Docker build context tarfile: %v", err)
	}

	defer func() {
		if closed {
			return
		}
		if err := tar.Close(); err != nil {
			log.Printf("Error closing build context: %v", err)
		}
	}()
	if err := tar.AddAll(fmt.Sprintf("%s/%s", gdc.RepoDirectory, gdc.DotfileDirectory), true); err != nil {
		return fmt.Errorf("Error adding Dotfiles to context file: %v", err)
	}
	if err := tar.AddAll(tmpDir, false); err != nil {
		return fmt.Errorf("Error adding Dockerfile to context file: %v", err)
	}
	if err := tar.Close(); err != nil {
		return fmt.Errorf("Error closing Docker build context file: %v", err)
	}
	closed = true

	dockerBuildContext, err := os.Open("/tmp/godot-buildcontext.tar")
	if err != nil {
		return fmt.Errorf("Error opening build context tarfile: %v", err)
	}
	defer func() {
		if err := dockerBuildContext.Close(); err != nil {
			log.Printf("Error closing docker build context file: %v", err)
		}
	}()

	cli, err := client.NewClientWithOpts(client.WithVersion("1.39"))
	if err != nil {
		return fmt.Errorf("Error initializing Docker client: %v", err)
	}
	options := types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Tags:           []string{fmt.Sprintf("%s:latest", gdc.ImageTag)},
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

	p := make([]byte, 512)
	for {
		n, err := buildResponse.Body.Read(p)
		if err != nil {
			if err == io.EOF {
				fmt.Println(string(p[:n]))
				break
			}
			return fmt.Errorf("Error reading from Docker build response: %v", err)
		}
		fmt.Println(string(p[:n]))
	}

	return nil
}

// godot builds and runs the docker image
func godot(u *url.URL, imageTag string, outputDir string) error {
	repoPath, p, err := fetchReadme(u)
	if err != nil {
		return fmt.Errorf("Error reading from Git repository: %v", err)
	}
	gdc, err := parseConfig(repoPath, p)
	if err != nil {
		return fmt.Errorf("Error parsing README.md configuration: %v", err)
	}
	gdc.ImageTag = imageTag
	gdc.OutputDirectory = outputDir
	renderedDockerfile, err := buildDockerfile(gdc, p)
	if err != nil {
		return fmt.Errorf("Error rendering Dockerfile.tmpl: %v", err)
	}
	gdc.DockerfileRendered = renderedDockerfile
	err = buildDockerimage(gdc)
	if err != nil {
		return fmt.Errorf("Error building Docker Image: %v", err)
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "godot"
	app.Usage = "godot run https://github.com/pmalmgren/godot"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name:    "run",
			Aliases: []string{"r"},
			Action: func(ctx *cli.Context) error {
				repoStr := ctx.Args().Get(len(ctx.Args()) - 1)
				u, err := url.Parse(repoStr)
				if err != nil {
					return err
				}
				if err := godot(u, ctx.String("image-tag"), ctx.String("output-dir")); err != nil {
					return err
				}
				return nil
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "image-tag",
					Value: "godot-dev",
					Usage: "image tag for the Docker image",
				},
				cli.StringFlag{
					Name:  "output-dir",
					Value: ".",
					Usage: "Output directory for Dockerfile",
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
