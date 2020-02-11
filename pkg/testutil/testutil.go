package testutil

import (
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/stretchr/testify/assert"
	"runtime"
	"testing"
	"time"
)

func TestListAfterCreate(t *testing.T, mng name_manager.NameManager) {
	defer reset(mng)

	lst, err := mng.List()
	assert.Nil(t, err)
	assert.Nil(t, lst)
}

func TestReleaseAfterCreate(t *testing.T, mng name_manager.NameManager) {
	defer reset(mng)

	err := mng.Release("foo", "bar")
	assert.Nil(t, err)
}

func TestAcquireTwiceForSameFamily(t *testing.T, mng name_manager.NameManager) {
	defer reset(mng)

	name0, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name0)

	name1, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "1", name1)
}

func TestAcquireForDifferentFamilies(t *testing.T, mng name_manager.NameManager) {
	defer reset(mng)

	nameFoo, err := mng.Acquire("foo")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameFoo)

	nameBar, err := mng.Acquire("bar")
	assert.Nil(t, err)
	assert.Equal(t, "0", nameBar)
}

func TestAcquireReleaseThenAcquireForAnotherFamily(t *testing.T, mng name_manager.NameManager) {
	defer reset(mng)

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
	defer reset(mng)

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
	defer reset(mng)

	wait := func(d time.Duration) {
		if mockClock != nil {
			mockClock.Add(d)
		}
	}

	var startTime time.Time
	if mockClock != nil {
		startTime = mockClock.Now().UTC()
	}

	_, err := mng.Acquire("foo")
	assert.Nil(t, err)

	wait(2 * time.Second)

	_, err = mng.Acquire("bar")
	assert.Nil(t, err)

	wait(2 * time.Second)

	_, err = mng.Acquire("foo")
	assert.Nil(t, err)

	wait(2 * time.Second)

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
			CreatedAt: startTime.Add(2 * time.Second).UTC(),
			UpdatedAt: startTime.Add(6 * time.Second).UTC(),
			Free:      false,
		},
		{
			Name:      "1",
			Family:    "foo",
			CreatedAt: startTime.Add(4 * time.Second).UTC(),
			Free:      true,
		},
	}

	if mockClock == nil {
		for i := range expectedNames {
			expectedNames[i].UpdatedAt = time.Unix(0, 0)
			expectedNames[i].CreatedAt = time.Unix(0, 0)
		}

		for i := range names {
			names[i].UpdatedAt = time.Unix(0, 0)
			names[i].CreatedAt = time.Unix(0, 0)
		}
	}

	assert.ElementsMatch(t, expectedNames, names)
}

func TestKeepAlive(t *testing.T, mng name_manager.NameManager, mockClock *clock.Mock) {
	defer reset(mng)

	wait := func(d time.Duration) {
		if mockClock != nil {
			mockClock.Add(d)
		} else {
			time.Sleep(d)
		}
	}

	name, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name)

	wait(1 * time.Second)

	// When the auto-release period is not past, a new name is acquired
	name, err = mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "1", name)

	wait(15 * time.Second)

	// After a certain time, the first name is auto-released and
	// acquired again
	name, err = mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, "0", name, "if the name is 2, it means auto-release did not work")
}

func TestHold(t *testing.T, mng name_manager.NameManager, mockClock *clock.Mock) {
	defer reset(mng)

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

	if mockClock != nil {
		mockClock.Add(7 * time.Second)
	} else {
		time.Sleep(7 * time.Second)
	}

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
	defer reset(mng)

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
	defer reset(mng)

	err := mng.TryAcquire("foo", "0")
	assert.Equal(t, err, name_manager.ErrNotExist)

	name, err := mng.Acquire("foo")
	assert.NoError(t, err)
	assert.Equal(t, name, "0")

	err = mng.TryAcquire("foo", "0")
	assert.Equal(t, err, name_manager.ErrInUse)
}

func TestTryHold(t *testing.T, mng name_manager.NameManager, mockClock *clock.Mock) {
	defer reset(mng)

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

	if mockClock != nil {
		mockClock.Add(7 * time.Second)
	} else {
		time.Sleep(7 * time.Second)
	}

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

func reset(mng name_manager.NameManager) {
	if runtime.GOOS == "windows" {
		// FIXME: On Windows, "reset" fails with
		// "The process cannot access the file because it is being used by another process."
		mng.Reset()
		return
	}
	if err := mng.Reset(); err != nil {
		panic(err)
	}
}
