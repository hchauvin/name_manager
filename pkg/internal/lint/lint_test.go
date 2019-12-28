// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package lint

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"testing"
)

var (
	backendPkgParent = "../.." // Relative to "lint" package
	backendPkgRE     = regexp.MustCompile(".*_backend")
	testRE           = regexp.MustCompile("func Test([^(]+)")
)

func TestAllBackendTestsAreImplemented(t *testing.T) {
	backends, err := listBackends()
	if err != nil {
		assert.FailNow(t, "cannot list backends", err)
	}

	if len(backends) == 0 {
		assert.FailNow(t, "expected at least one backend")
	}

	expectedTests, err := listTests(filepath.Join(backendPkgParent, "testutil/testutil.go"))
	if err != nil {
		assert.FailNow(t, "cannot list expected tests", err)
	}

	for _, backend := range backends {
		testsPath := filepath.Join(backend, filepath.Base(backend)+"_test.go")
		tests, err := listTests(testsPath)
		assert.NoError(t, err, "backend %s", backend)

		for _, test := range expectedTests {
			assert.Contains(t, tests, test, "backend %s: expected to have test %s", backend, test)
		}
	}
}

func listBackends() ([]string, error) {
	files, err := ioutil.ReadDir(backendPkgParent)
	if err != nil {
		return nil, err
	}

	var backends []string
	for _, f := range files {
		if !backendPkgRE.MatchString(f.Name()) {
			continue
		}
		backends = append(backends, filepath.Join(backendPkgParent, f.Name()))
	}

	return backends, nil
}

func listTests(path string) ([]string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	matches := testRE.FindAllSubmatch(content, -1)
	tests := make([]string, len(matches))
	for i, m := range matches {
		tests[i] = string(m[1])
	}

	return tests, nil
}
