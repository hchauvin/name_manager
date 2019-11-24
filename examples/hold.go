// hold tests/demonstrates the "name_manager hold" command.
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func nameManagerCommand(backend string, args ...string) *exec.Cmd {
	fullArgs := make([]string, 0, len(args)+1)
	fullArgs = append(fullArgs, "--backend="+backend)
	fullArgs = append(fullArgs, args...)
	return exec.Command("./bin/name_manager", fullArgs...)
}

type releaseFunc func() error

func hold(backend, family string) (string, releaseFunc, error) {
	cmd := nameManagerCommand(backend, "hold", family)
	cmd.Stderr = os.Stderr
	r, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("could not start name_manager process: %v", err)
	}
	releaseFunc := func() error {
		return cmd.Process.Kill()
	}
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return "", nil, fmt.Errorf("unexpected EOF")
	}
	return strings.TrimSpace(string(scanner.Bytes())), releaseFunc, nil
}

func protocol() error {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		return err
	}
	defer tmpfile.Close()
	backend := "local://" + tmpfile.Name() + ";autoReleaseAfter=5s"

	name1, release1, err := hold(backend, "foo")
	if err != nil {
		return err
	}

	if err := release1(); err != nil {
		return err
	}

	name1, release1, err = hold(backend, "foo")
	if err != nil {
		return err
	}

	name2, release2, err := hold(backend, "foo")
	if err != nil {
		return err
	}

	if name2 == name1 {
		return fmt.Errorf("a new name should have been acquired; instead got: '%s'", name1)
	}

	if err := release2(); err != nil {
		return err
	}

	if err := release1(); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := protocol(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
