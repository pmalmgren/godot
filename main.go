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
	ImageTag           string   `yaml:"image-tag"`
	EntryPoint         string   "yaml:`entrypoint`"
	RepoDirectory      string
	DockerfileRendered string
}

// fetchReadme grabs the README.md from the repository
func fetchReadme(u *url.URL) (string, string, error) {
	r := fmt.Sprintf("/tmp%s", u.EscapedPath())
	if err := os.MkdirAll(r, os.ModePerm); err != nil {
		return "", "", err
	}

	_, err := git.PlainClone(r, false, &git.CloneOptions{
		URL:   u.String(),
		Depth: 1,
	})

	if err != nil && err != git.ErrRepositoryAlreadyExists {
		return "", "", err
	}

	readmePath := fmt.Sprintf("%s/README.md", r)

	if _, err := os.Stat(readmePath); err != nil {
		return "", "", err
	}

	return readmePath, r, nil
}

// parseConfig parses out the godot configuration from a README file
func parseConfig(readmePath string, repoPath string) (*GoDotConfig, error) {
	f, err := os.OpenFile(readmePath, os.O_RDONLY, os.ModePerm)

	if err != nil {
		return nil, err
	}

	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println("Error closing README.md")
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
		return nil, err
	}
	gdc.RepoDirectory = repoPath
	return &gdc, nil
}

// validates & applies the configuration to the template
func buildDockerfile(gdc *GoDotConfig, repoPath string) (string, error) {
	t, err := template.ParseFiles("Dockerfile.tmpl")
	if err != nil {
		return "", err
	}

	f, err := os.Create("Dockerfile.godot")
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing Dockerfile")
		}
	}()
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	err = t.Execute(&tpl, gdc)

	if err != nil {
		return "", err
	}
	renderedTemplate := tpl.String()

	f.Write([]byte(renderedTemplate))

	return renderedTemplate, nil
}

// optionally, build the docker image
func buildDockerimage(gdc *GoDotConfig) error {
	tmpDir, err := ioutil.TempDir("/tmp/", "godot-build-context")
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Error removing temporary directory: %v", err)
		}
	}()
	err = ioutil.WriteFile(fmt.Sprintf("%s/Dockerfile", tmpDir), []byte(gdc.DockerfileRendered), 0666)
	if err != nil {
		return err
	}

	tar := new(archivex.TarFile)
	tar.Create("/tmp/godot-buildcontext.tar")

	defer func() {
		if err := tar.Close(); err != nil {
			log.Printf("Error closing build context: %v", err)
		}
	}()
	if err := tar.AddAll(fmt.Sprintf("%s/%s", gdc.RepoDirectory, gdc.DotfileDirectory), true); err != nil {
		return err
	}
	if err := tar.AddAll(tmpDir, false); err != nil {
		return err
	}
	tar.Close()

	dockerBuildContext, err := os.Open("/tmp/godot-buildcontext.tar")
	defer dockerBuildContext.Close()

	cli, err := client.NewClientWithOpts(client.WithVersion("1.39"))
	if err != nil {
		return err
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
		return err
	}
	defer buildResponse.Body.Close()

	p := make([]byte, 512)
	for {
		n, err := buildResponse.Body.Read(p)
		if err != nil {
			if err == io.EOF {
				fmt.Println(string(p[:n]))
				break
			}
			fmt.Printf("Error: %v\n", err)
			return err
		}
		fmt.Println(string(p[:n]))
	}

	return nil
}

// godot builds and runs the docker image
func godot(u *url.URL) error {
	r, p, err := fetchReadme(u)
	if err != nil {
		return err
	}
	gdc, err := parseConfig(r, p)
	if err != nil {
		return err
	}
	renderedDockerfile, err := buildDockerfile(gdc, p)
	if err != nil {
		return err
	}
	gdc.DockerfileRendered = renderedDockerfile
	err = buildDockerimage(gdc)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "godot"
	app.Usage = "godot run https://github.com/pmalmgren/godot"
	app.Version = "0.0.0"

	app.Commands = []cli.Command{
		{
			Name:    "run",
			Aliases: []string{"r"},
			Action: func(ctx *cli.Context) error {
				repoStr := ctx.Args().Get(0)
				u, err := url.Parse(repoStr)
				if err != nil {
					return err
				}
				if err := godot(u); err != nil {
					return err
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
