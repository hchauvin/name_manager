package local_backend

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandHome(t *testing.T) {
	{
		path, err := expandHome("foo")
		assert.Nil(t, err)
		assert.Equal(t, "foo", path)
	}

	{
		path, err := expandHome("~/foo")
		assert.Nil(t, err)
		assert.NotEqual(t, "~/foo", path)
		assert.True(t, strings.HasSuffix(path, string(filepath.Separator)+"foo"), "path must end with /foo")
	}

	{
		path, err := expandHome("~")
		assert.Nil(t, err)
		assert.Equal(t, "~", path)
	}
}
