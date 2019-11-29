// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package name_manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testNameManager struct {
	backendURL string
}

func TestCreateFromURL(t *testing.T) {
	backend := Backend{
		Protocol:    "backend",
		Description: "foo",
		CreateNameManager: func(backendURL string) (NameManager, error) {
			return &testNameManager{
				backendURL: backendURL,
			}, nil
		},
	}
	RegisterBackend(backend)

	tnm, err := CreateFromURL("backend://my/url")
	assert.Nil(t, err)
	assert.Equal(t, tnm.(*testNameManager).backendURL, "my/url")

	name, err := tnm.Acquire("")
	assert.Nil(t, err)
	assert.Equal(t, "foo", name)
}

func (tnm *testNameManager) Hold(family string) (string, ReleaseFunc, error) {
	return "foo", nil, nil
}

func (tnm *testNameManager) Acquire(family string) (string, error) {
	return "foo", nil
}

func (tnm *testNameManager) KeepAlive(family, name string) error {
	return nil
}

func (tnm *testNameManager) Release(family, name string) error {
	return nil
}

func (tnm *testNameManager) TryAcquire(family, name string) error {
	return nil
}

func (tnm *testNameManager) TryHold(family, name string) (ReleaseFunc, error) {
	return nil, nil
}

func (tnm *testNameManager) List() ([]Name, error) {
	return nil, nil
}

func (tnm *testNameManager) Reset() error {
	return nil
}
