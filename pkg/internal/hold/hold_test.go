// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package hold

import (
	"errors"
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestKeepAliveErrorOnHold(t *testing.T) {
	hold := &Hold{
		Manager:           &testNameManager{},
		Clock:             clock.New(),
		KeepAliveInterval: 1 * time.Millisecond,
	}

	_, errc, releaseFunc, err := hold.Hold("foo")
	assert.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	err = releaseFunc()
	assert.NoError(t, err)

	select {
	case err := <-errc:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keep-alive error")
	default:
		assert.Fail(t, "expected a detached error")
	}
}

func TestKeepAliveErrorOnTryHold(t *testing.T) {
	hold := &Hold{
		Manager:           &testNameManager{},
		Clock:             clock.New(),
		KeepAliveInterval: 1 * time.Millisecond,
	}

	errc, releaseFunc, err := hold.TryHold("foo", "bar")
	assert.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	err = releaseFunc()
	assert.NoError(t, err)

	select {
	case err := <-errc:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keep-alive error")
	default:
		assert.Fail(t, "expected a detached error")
	}
}

type testNameManager struct{}

func (tnm *testNameManager) Hold(family string) (string, <-chan error, name_manager.ReleaseFunc, error) {
	return "foo", nil, nil, nil
}

func (tnm *testNameManager) Acquire(family string) (string, error) {
	return "foo", nil
}

func (tnm *testNameManager) KeepAlive(family, name string) error {
	return errors.New("keep-alive error")
}

func (tnm *testNameManager) Release(family, name string) error {
	return nil
}

func (tnm *testNameManager) TryAcquire(family, name string) error {
	return nil
}

func (tnm *testNameManager) TryHold(family, name string) (<-chan error, name_manager.ReleaseFunc, error) {
	return nil, nil, nil
}

func (tnm *testNameManager) List() ([]name_manager.Name, error) {
	return nil, nil
}

func (tnm *testNameManager) Reset() error {
	return nil
}
