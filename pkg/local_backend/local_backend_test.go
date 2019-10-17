package local_backend

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/stretchr/testify/assert"
)

func TestListAfterCreate(t *testing.T) {
	mng := createTestNameManager(t)
	defer mng.Reset()

	lst, err := mng.List()
	assert.Nil(t, err)
	assert.Nil(t, lst)
}

func TestReleaseAfterCreate(t *testing.T) {
	mng := createTestNameManager(t)
	defer mng.Reset()

	err := mng.Release("foo", "bar")
	assert.Nil(t, err)
}

func TestAcquireTwiceForSameFamily(t *testing.T) {
	mng := createTestNameManager(t)
	defer mng.Reset()

	name0, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "0", name0)

	name1, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "1", name1)
}

func TestAcquireForDifferentFamilies(t *testing.T) {
	mng := createTestNameManager(t)
	defer mng.Reset()

	nameFoo, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameFoo)

	nameBar, err := mng.Acquire("bar")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameBar)
}

func TestAcquireAcquireReleaseAcquireAcquire(t *testing.T) {
	mng := createTestNameManager(t)
	defer mng.Reset()

	name0, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "0", name0)

	name1, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "1", name1)

	err = mng.Release("foo", "0")
	assert.Nil(t, err)

	name0Again, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "0", name0Again)

	name2, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "2", name2)
}

func TestList(t *testing.T) {
	mng := createTestNameManager(t)
	defer mng.Reset()

	mockClock := clock.NewMock()
	mng.(*localBackend).clock = mockClock
	startTime := mockClock.Now().UTC()

	_, err := mng.Acquire("foo")
	assert.Nil(t, err)

	mockClock.Add(2 * time.Hour)

	_, err = mng.Acquire("bar")
	assert.Nil(t, err)

	mockClock.Add(2 * time.Hour)

	_, err = mng.Acquire("foo")
	assert.Nil(t, err)

	mockClock.Add(2 * time.Hour)

	err = mng.Release("foo", "1")
	assert.Nil(t, err)

	err = mng.Release("bar", "0")
	assert.Nil(t, err)

	_, err = mng.Acquire("bar")
	assert.Nil(t, err)

	names, err := mng.List()
	assert.Nil(t, err)
	expectedNames := []name_manager.Name{
		{
			Name:      "0",
			Family:    "foo",
			CreatedAt: startTime,
			UpdatedAt: startTime,
			Free:      false,
		},
		{
			Name:      "0",
			Family:    "bar",
			CreatedAt: startTime.Add(2 * time.Hour).UTC(),
			UpdatedAt: startTime.Add(6 * time.Hour).UTC(),
			Free:      false,
		},
		{
			Name:      "1",
			Family:    "foo",
			CreatedAt: startTime.Add(4 * time.Hour).UTC(),
			UpdatedAt: startTime.Add(4 * time.Hour).UTC(),
			Free:      true,
		},
	}
	assert.ElementsMatch(t, expectedNames, names)
}

func createTestNameManager(t *testing.T) name_manager.NameManager {
	tmpfile, err := ioutil.TempFile("", "example")
	assert.Nil(t, err)
	manager, err := createNameManager(tmpfile.Name())
	assert.Nil(t, err)
	return manager
}
