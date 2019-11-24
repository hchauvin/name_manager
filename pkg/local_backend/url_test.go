package local_backend

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestParseBackendURL(t *testing.T) {
	path, options, err := parseBackendURL("./foo")
	assert.NoError(t, err)
	assert.Equal(t, "./foo", path)
	assert.Equal(t, 0*time.Second, options.autoReleaseAfter)

	path, options, err = parseBackendURL("./foo;autoReleaseAfter=15s")
	assert.NoError(t, err)
	assert.Equal(t, "./foo", path)
	assert.Equal(t, 15*time.Second, options.autoReleaseAfter)

	_, _, err = parseBackendURL("./foo;__invalid__")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "options must have format")

	_, _, err = parseBackendURL("./foo;__unknown__=bar")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized option \"__unknown__\"")

	_, _, err = parseBackendURL("./foo;autoReleaseAfter=__invalid__")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse duration for autoReleaseAfter")
}
