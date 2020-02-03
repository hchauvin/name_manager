// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

// hold_command tests/demonstrates the "name_manager hold" command when
// it wraps a command.
package hold_command

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// backend contains the name_manager backend URL, after initialization.
var backend string

// commandPath contains the path to the "hold_command" command line,
// after initialization.
var commandPath string

const commandPkg = "github.com/hchauvin/name_manager/examples/hold_command/command"

func TestMain(m *testing.M) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		panic(err.Error())
	}
	defer tmpfile.Close()
	backend = "local://" + tmpfile.Name() + ";autoReleaseAfter=5s"

	var execPattern string
	if runtime.GOOS == "windows" {
		execPattern = "command_*.exe"
	} else {
		execPattern = "command_*"
	}
	commandTmpfile, err := ioutil.TempFile("", execPattern)
	if err != nil {
		panic(err.Error())
	}
	defer commandTmpfile.Close()
	commandPath = commandTmpfile.Name()

	if err := goBuild(commandPkg, commandPath); err != nil {
		panic(err.Error())
	}

	os.Exit(m.Run())
}

// TestOutput tests that "hold" pipes streams as expected.
func TestOutput(t *testing.T) {
	expected := &behavior{
		stdout: "__out__",
		stderr: "__err__",
	}

	actual, err := holdStub(backend, "foo", expected)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

// TestOutput tests that "hold" passes on a non-zero exit code.
func TestExitCode(t *testing.T) {
	expected := &behavior{
		exitCode: 28,
	}

	actual, err := holdStub(backend, "foo", expected)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

// TestCommandNotFound tests that "hold" fails when the command cannot be found.
func TestCommandNotFound(t *testing.T) {
	bh, err := hold(backend, "foo", []string{"__not_found__"}, []string{})
	assert.NoError(t, err)

	assert.Equal(t, 1, bh.exitCode)
	assert.Contains(t, bh.stderr, "executable file not found")
}

func nameManagerCommand(backend string, args ...string) *exec.Cmd {
	fullArgs := make([]string, 0, len(args)+1)
	fullArgs = append(fullArgs, "--backend="+backend)
	fullArgs = append(fullArgs, args...)
	commandName := "../../bin/name_manager"
	if runtime.GOOS == "windows" {
		commandName += ".exe"
	}
	return exec.Command(commandName, fullArgs...)
}

type behavior struct {
	stdout   string
	stderr   string
	exitCode int
}

func holdStub(backend, family string, expected *behavior) (*behavior, error) {
	return hold(
		backend,
		family,
		[]string{
			commandPath, "__expected__",
		},
		[]string{
			"STDOUT=" + expected.stdout,
			"STDERR=" + expected.stderr,
			fmt.Sprintf("EXIT_CODE=%d", expected.exitCode),
		},
	)
}

func hold(backend, family string, command []string, env []string) (*behavior, error) {
	args := []string{"hold", family}
	args = append(args, command...)
	cmd := nameManagerCommand(backend, args...)
	cmd.Env = append(os.Environ(), env...)

	rout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	rerr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	bh := &behavior{}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	if b, err := ioutil.ReadAll(rout); err != nil {
		return nil, err
	} else {
		bh.stdout = string(b)
	}

	if b, err := ioutil.ReadAll(rerr); err != nil {
		return nil, err
	} else {
		bh.stderr = string(b)
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			bh.exitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	return bh, nil
}
