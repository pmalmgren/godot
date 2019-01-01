// godot
// https://github.com/pmalmgren/godot
//
// Copyright © 2018 Peter Malmgren <me@petermalmgren.com>
// Distributed under the MIT License.
// See README.md for details.
//

package conf

import "net/url"

const (
	confHeader         = "## godot configuration"
	confBoundaryToken  = "```"
	dockerfileTemplate = "Dockerfile.tmpl"
)

// GoDotConfig contains the relevant configuration to pass to the Dockerfile template
type GoDotConfig struct {
	Username           string   `yaml:"username"`
	DotfileDirectory   string   `yaml:"dotfile-directory"`
	Packages           []string `yaml:"packages"`
	SystemSetup        []string `yaml:"system-setup"`
	UserSetup          []string `yaml:"user-setup"`
	EntryPoint         string   `yaml:"entrypoint"`
	ImageTag           string   `yaml:"image-tag"`
	OutputDirectory    string
	RepoDirectory      string
	DockerfileRendered string
}

// Repository encapsulates functionality around access to files from a Git repository
type Repository struct {
	RepoDirectory string
	Remote        *url.URL
}
