// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package rest_backend

import (
	"fmt"
	"github.com/benbjohnson/clock"
	_ "github.com/hchauvin/name_manager/pkg/local_backend"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	testserver "github.com/hchauvin/name_manager/pkg/server/test"
	"github.com/hchauvin/name_manager/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestListAfterCreate(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestListAfterCreate(t, mng)
}

func TestReleaseAfterCreate(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestReleaseAfterCreate(t, mng)
}

func TestAcquireTwiceForSameFamily(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestAcquireTwiceForSameFamily(t, mng)
}

func TestAcquireForDifferentFamilies(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestAcquireForDifferentFamilies(t, mng)
}

func TestAcquireReleaseThenAcquireForAnotherFamily(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestAcquireReleaseThenAcquireForAnotherFamily(t, mng)
}

func TestAcquireAcquireReleaseAcquireAcquire(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestAcquireAcquireReleaseAcquireAcquire(t, mng)
}

func TestList(t *testing.T) {
	mng, ts := createTestNameManager(t, 0)
	mockClock := clock.NewMock()
	mng.(*restBackend).clock = mockClock
	ts.MockClock(mockClock)
	testutil.TestList(t, mng, mockClock)
}

func TestKeepAlive(t *testing.T) {
	mng, ts := createTestNameManager(t, 5)
	mockClock := clock.NewMock()
	mng.(*restBackend).clock = mockClock
	ts.MockClock(mockClock)
	testutil.TestKeepAlive(t, mng, mockClock)
}

func TestHold(t *testing.T) {
	mng, _ := createTestNameManager(t, 5)
	mockClock := clock.NewMock()
	mng.(*restBackend).clock = mockClock
	testutil.TestHold(t, mng, mockClock)
}

func TestTryAcquire(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestTryAcquire(t, mng)
}

func TestTryAcquireErrors(t *testing.T) {
	mng, _ := createTestNameManager(t, 0)
	testutil.TestTryAcquireErrors(t, mng)
}

func TestTryHold(t *testing.T) {
	mng, ts := createTestNameManager(t, 5)
	mockClock := clock.NewMock()
	mng.(*restBackend).clock = mockClock
	ts.MockClock(mockClock)
	testutil.TestTryHold(t, mng, mockClock)
}

func createTestNameManager(
	t *testing.T,
	autoReleaseAfter int,
) (name_manager.NameManager, *testserver.TestServer) {
	ts, err := testserver.New(autoReleaseAfter)
	assert.Nil(t, err)

	url := "localhost:%d"
	if autoReleaseAfter > 0 {
		url = url + fmt.Sprintf(";keepAliveInterval=%ds", autoReleaseAfter/3)
	}
	manager, err := createNameManager(fmt.Sprintf(url, ts.Port))
	assert.Nil(t, err)

	manager.(*restBackend).resetHook = ts.Clean
	return manager, ts
}
