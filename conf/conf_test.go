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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The boundary is ```, which can't be escaped in a Go multi-line string
var readme string = `# Test README

## godot configuration

hey nothing here should matter!
` + "```" + `
username: test-user
dotfile-directory: test-dotfile-directory
entrypoint: test-entrypoint
image-tag: test-dev-env

packages:
  - neovim
  - git

# system-setup runs as root, define volumes etc.
system-setup:
  - RUN ls
  - RUN touch system-setup

# user-setup runs as the user defined above in username.
user-setup:
  - RUN mkdir user-setup
  - RUN cd user-setup
` + "```"

func TestConfigFromReadme(t *testing.T) {
	err := ioutil.WriteFile("README.md", []byte(readme), 0666)
	if err != nil {
		t.Fatalf("Error creating test README.md file: %v", err)
	}
	defer func() {
		if err := os.Remove("README.md"); err != nil {
			t.Logf("Error removing README.md file: %v", err)
		}
	}()
	dir, err := filepath.Abs(filepath.Dir("README.md"))
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	r := &Repository{RepoDirectory: dir}

	actual, err := ConfigFromReadme(r)

	if err != nil {
		t.Fatalf("Error creating readme: %v", err)
	}

	expected := &GoDotConfig{
		Username:           "test-user",
		DotfileDirectory:   "test-dotfile-directory",
		Packages:           []string{"neovim", "git"},
		SystemSetup:        []string{"RUN ls", "RUN touch system-setup"},
		UserSetup:          []string{"RUN mkdir user-setup", "RUN cd user-setup"},
		EntryPoint:         "test-entrypoint",
		ImageTag:           "test-dev-env",
		OutputDirectory:    "",
		RepoDirectory:      dir,
		DockerfileRendered: "",
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected != actual.\n%+v\n!=\n%+v", expected, actual)
	}
}
