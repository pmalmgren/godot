//
// godot
// https://github.com/pmalmgren/godot
//
// Copyright © 2018 Peter Malmgren <me@petermalmgren.com>
// Distributed under the MIT License.
// See README.md for details.
//

package conf

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

func parseReadme(path string) (string, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)

	if err != nil {
		return "", fmt.Errorf("Error opening README.md: %v", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing README.md: %v", err)
		}
	}()

	sc := bufio.NewScanner(f)
	var raw string
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
			raw += token
			raw += "\n"
		}
	}

	if !sawHeader {
		return "", fmt.Errorf("Your README.md needs a `## godot configuration` header")
	}

	if !insideConfig {
		return "", fmt.Errorf("Your README.md needs a YAML code block with the configuration")
	}

	return raw, nil
}

// ConfigFromReadme parses the README.md and reads it into a `GoDotConfig` object.
func ConfigFromReadme(r *Repository) (*GoDotConfig, error) {
	readmePath, err := r.GetFilePath("README.md")
	if err != nil {
		return nil, fmt.Errorf("README.md does not exist: %v", err)
	}

	raw, err := parseReadme(readmePath)
	if err != nil {
		return nil, fmt.Errorf("Invalid Godot configuration: %v", err)
	}

	var gdc GoDotConfig
	gdc.RepoDirectory = r.RepoDirectory
	if err := yaml.Unmarshal([]byte(raw), &gdc); err != nil {
		return nil, fmt.Errorf("Error reading repository configuration: %v", err)
	}
	gdc.DockerfileRendered, err = BuildDockerfile(&gdc)
	if err != nil {
		return nil, fmt.Errorf("Error compiling Dockerfile template: %v", err)
	}
	return &gdc, nil
}

// BuildDockerfile applies a GoDotConfig object to the Dockerfile.tmpl file
func BuildDockerfile(gdc *GoDotConfig) (string, error) {
	t, err := template.ParseFiles(dockerfileTemplate)
	if err != nil {
		return "", fmt.Errorf("Error parsing template file: %v", err)
	}
	var tpl bytes.Buffer
	err = t.Execute(&tpl, gdc)

	if err != nil {
		return "", fmt.Errorf("Error rendering Dockerfile.tmpl: %v", err)
	}
	return tpl.String(), nil
}
