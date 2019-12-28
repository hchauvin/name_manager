package testutil

import (
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestListAfterCreate(t *testing.T, mng name_manager.NameManager) {
	defer mng.Reset()

	lst, err := mng.List()
	assert.Nil(t, err)
	assert.Nil(t, lst)
}

func TestReleaseAfterCreate(t *testing.T, mng name_manager.NameManager) {
	defer mng.Reset()

	err := mng.Release("foo", "bar")
	assert.Nil(t, err)
}

func TestAcquireTwiceForSameFamily(t *testing.T, mng name_manager.NameManager) {
	defer mng.Reset()

	name0, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name0)

	name1, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "1", name1)
}

func TestAcquireForDifferentFamilies(t *testing.T, mng name_manager.NameManager) {
	defer mng.Reset()

	nameFoo, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameFoo)

	nameBar, err := mng.Acquire("bar")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameBar)
}

func TestAcquireReleaseThenAcquireForAnotherFamily(t *testing.T, mng name_manager.NameManager) {
	defer mng.Reset()

	nameFoo, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameFoo)

	err = mng.Release("foo", "0")
	assert.Nil(t, err)

	nameBar, err := mng.Acquire("bar")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameBar)
}

func TestAcquireAcquireReleaseAcquireAcquire(t *testing.T, mng name_manager.NameManager) {
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

func TestList(t *testing.T, mng name_manager.NameManager, mockClock *clock.Mock) {
	defer mng.Reset()

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
			Free:      true,
		},
	}
	assert.ElementsMatch(t, expectedNames, names)
}

func TestKeepAlive(t *testing.T, mng name_manager.NameManager, mockClock *clock.Mock) {
	defer mng.Reset()

	name, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name)

	mockClock.Add(1 * time.Second)

	// When the auto-release period is not past, a new name is acquired
	name, err = mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "1", name)

	mockClock.Add(15 * time.Second)

	// After a certain time, the first name is auto-released and
	// acquired again
	name, err = mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name, "if the name is 2, it means auto-release did not work")
}

func TestHold(t *testing.T, mng name_manager.NameManager, mockClock *clock.Mock) {
	defer mng.Reset()

	name, errc, release, err := mng.Hold("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name)
	go func() {
		for err := range errc {
			assert.Fail(t, "detached error", err)
		}
	}()

	names, err := mng.List()
	assert.NoError(t, err)
	assert.Len(t, names, 1)
	assert.Equal(t, "foo", names[0].Family)
	assert.Equal(t, "0", names[0].Name)
	assert.Equal(t, false, names[0].Free)

	mockClock.Add(7 * time.Second)

	// The name is still there, and not free, past the auto-release
	// period
	names, err = mng.List()
	assert.NoError(t, err)
	assert.Len(t, names, 1)
	assert.Equal(t, "foo", names[0].Family)
	assert.Equal(t, "0", names[0].Name)
	assert.Equal(t, false, names[0].Free)

	err = release()
	assert.NoError(t, err)

	// The name has been freed
	names, err = mng.List()
	assert.NoError(t, err)
	assert.Len(t, names, 1)
	assert.Equal(t, "foo", names[0].Family)
	assert.Equal(t, "0", names[0].Name)
	assert.Equal(t, true, names[0].Free)
}

func TestTryAcquire(t *testing.T, mng name_manager.NameManager) {
	defer mng.Reset()

	name, err := mng.Acquire("foo")
	assert.NoError(t, err)
	if name != "0" {
		t.Fatal()
	}

	err = mng.TryAcquire("foo", "0")
	assert.Error(t, err)

	err = mng.Release("foo", "0")
	assert.NoError(t, err)

	err = mng.TryAcquire("foo", "0")
	assert.NoError(t, err)

	err = mng.TryAcquire("foo", "0")
	assert.Error(t, err)
}

func TestTryAcquireErrors(t *testing.T, mng name_manager.NameManager) {
	defer mng.Reset()

	err := mng.TryAcquire("foo", "0")
	assert.Equal(t, err, name_manager.ErrNotExist)

	name, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, name, "0")

	err = mng.TryAcquire("foo", "0")
	assert.Equal(t, err, name_manager.ErrInUse)
}

func TestTryHold(t *testing.T, mng name_manager.NameManager, mockClock *clock.Mock) {
	defer mng.Reset()

	name, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name)

	err = mng.Release("foo", "0")
	assert.NoError(t, err)

	errc, release, err := mng.TryHold("foo", "0")
	assert.NoError(t, err)
	go func() {
		for err := range errc {
			assert.Fail(t, "detached error", err)
		}
	}()

	names, err := mng.List()
	assert.NoError(t, err)
	assert.Len(t, names, 1)
	assert.Equal(t, "foo", names[0].Family)
	assert.Equal(t, "0", names[0].Name)
	assert.Equal(t, false, names[0].Free)

	mockClock.Add(7 * time.Second)

	// The name is still there, and not free, past the auto-release
	// period
	names, err = mng.List()
	assert.NoError(t, err)
	assert.Len(t, names, 1)
	assert.Equal(t, "foo", names[0].Family)
	assert.Equal(t, "0", names[0].Name)
	assert.Equal(t, false, names[0].Free)

	err = release()
	assert.NoError(t, err)

	// The name has been freed
	names, err = mng.List()
	assert.NoError(t, err)
	assert.Len(t, names, 1)
	assert.Equal(t, "foo", names[0].Family)
	assert.Equal(t, "0", names[0].Name)
	assert.Equal(t, true, names[0].Free)
}
