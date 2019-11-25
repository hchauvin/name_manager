// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package local_backend

import (
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/hchauvin/name_manager/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"strings"
	"testing"
)

func TestListAfterCreate(t *testing.T) {
	testutil.TestListAfterCreate(t, createTestNameManager(t))
}

func TestReleaseAfterCreate(t *testing.T) {
	testutil.TestReleaseAfterCreate(t, createTestNameManager(t))
}

func TestAcquireTwiceForSameFamily(t *testing.T) {
	testutil.TestAcquireTwiceForSameFamily(t, createTestNameManager(t))
}

func TestAcquireForDifferentFamilies(t *testing.T) {
	testutil.TestAcquireForDifferentFamilies(t, createTestNameManager(t))
}

func TestAcquireReleaseThenAcquireForAnotherFamily(t *testing.T) {
	testutil.TestAcquireReleaseThenAcquireForAnotherFamily(t, createTestNameManager(t))
}

func TestAcquireAcquireReleaseAcquireAcquire(t *testing.T) {
	testutil.TestAcquireAcquireReleaseAcquireAcquire(t, createTestNameManager(t))
}

func TestList(t *testing.T) {
	mng := createTestNameManager(t)
	mockClock := clock.NewMock()
	mng.(*localBackend).clock = mockClock
	testutil.TestList(t, mng, mockClock)
}

func TestKeepAlive(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	mockClock := clock.NewMock()
	mng.(*localBackend).clock = mockClock
	testutil.TestKeepAlive(t, mng, mockClock)
}

func TestHold(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	mockClock := clock.NewMock()
	mng.(*localBackend).clock = mockClock
	testutil.TestHold(t, mng, mockClock)
}

func createTestNameManager(t *testing.T, options ...string) name_manager.NameManager {
	tmpfile, err := ioutil.TempFile("", "example")
	assert.Nil(t, err)
	var url strings.Builder
	url.WriteString(tmpfile.Name())
	for _, option := range options {
		url.WriteRune(';')
		url.WriteString(option)
	}
	manager, err := createNameManager(url.String())
	assert.Nil(t, err)
	return manager
}
