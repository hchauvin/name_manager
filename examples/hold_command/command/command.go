// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

// command implements a cross-platform stub for commands, to test/demonstrate
// when "name_manager hold" wraps around a command.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("expected two cmd arguments, got: %v", os.Args)
	}
	if os.Args[1] != "__expected__" {
		log.Fatalf("unexpected cmd argument: '%s'", os.Args[1])
	}

	fmt.Print(os.Getenv("STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("STDERR"))

	exitCodeStr := os.Getenv("EXIT_CODE")
	exitCode, err := strconv.Atoi(exitCodeStr)
	if err != nil {
		log.Fatalf("expected EXIT_CODE to be numeric; got: '%s'", exitCodeStr)
	}

	os.Exit(exitCode)
}
