// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package server_test

import (
	"fmt"
	_ "github.com/hchauvin/name_manager/pkg/local_backend"
	testserver "github.com/hchauvin/name_manager/pkg/server/test"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestHealth(t *testing.T) {
	ts, err := testserver.New(0)
	assert.NoError(t, err)
	defer ts.Clean()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", ts.Port))
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 200)
}

func TestPathNotFound(t *testing.T) {
	ts, err := testserver.New(0)
	assert.NoError(t, err)
	defer ts.Clean()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/__invalid__", ts.Port))
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 404)
}
