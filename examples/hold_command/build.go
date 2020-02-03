// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package hold_command

import (
	"os"
	"os/exec"
)

// goBuild executes "go build".
func goBuild(pkg string, outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath, pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
