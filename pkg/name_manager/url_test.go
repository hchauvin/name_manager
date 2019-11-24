// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package name_manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseURL(t *testing.T) {
	backendProtocol, backendUrl, err := parseURL("backendProtocol://backend/url")
	assert.Nil(t, err)
	assert.Equal(t, "backendProtocol", backendProtocol)
	assert.Equal(t, "backend/url", backendUrl)
}

func testParseMalformedURL(t *testing.T) {
	_, _, err := parseURL("malformed")
	assert.NotNil(t, err)
}
