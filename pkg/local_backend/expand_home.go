// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package local_backend

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
)

// Expands the tilde in a path to the current user's home directory.
func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("cannot expand home in flock directory: %v", err)
	}
	dir := usr.HomeDir
	return filepath.Join(dir, path[2:]), nil
}
