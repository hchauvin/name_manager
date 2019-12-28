package rest_backend

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestParseBackendURL(t *testing.T) {
	path, options, err := parseBackendURL("domain.test")
	assert.NoError(t, err)
	assert.Equal(t, "http://domain.test", path)
	assert.Equal(t, 0*time.Second, options.keepAliveInterval)

	path, options, err = parseBackendURL("domain.test;keepAliveInterval=15s")
	assert.NoError(t, err)
	assert.Equal(t, "http://domain.test", path)
	assert.Equal(t, 15*time.Second, options.keepAliveInterval)

	_, _, err = parseBackendURL("domain.test;__invalid__")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "options must have format")

	_, _, err = parseBackendURL("domain.test;__unknown__=bar")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized option \"__unknown__\"")

	_, _, err = parseBackendURL("domain.test;keepAliveInterval=__invalid__")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse duration for keepAliveInterval")
}
