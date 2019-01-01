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
	"fmt"
	"os"

	git "gopkg.in/src-d/go-git.v4"
)

// Pull clones a Git repository into a directory
func (r *Repository) Pull() error {
	_, err := git.PlainClone(r.RepoDirectory, false, &git.CloneOptions{
		URL:   r.Remote.String(),
		Depth: 1,
	})

	if err != nil && err != git.ErrRepositoryAlreadyExists {
		return fmt.Errorf("Error cloning repository: %v", err)
	}

	return nil
}

// GetFile checks to see if a file or path exists
func (r *Repository) GetFilePath(path string) (string, error) {
	fullPath := fmt.Sprintf("%s/%s", r.RepoDirectory, path)
	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("%s does not exist.", fullPath)
	}
	return fullPath, nil
}
