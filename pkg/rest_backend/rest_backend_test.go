// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package rest_backend

import (
	"fmt"
	_ "github.com/hchauvin/name_manager/pkg/local_backend"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	testserver "github.com/hchauvin/name_manager/pkg/server/test"
	"github.com/hchauvin/name_manager/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestListAfterCreate(t *testing.T) {
	testutil.TestListAfterCreate(t, createTestNameManager(t, 0))
}

func TestReleaseAfterCreate(t *testing.T) {
	testutil.TestReleaseAfterCreate(t, createTestNameManager(t, 0))
}

func TestAcquireTwiceForSameFamily(t *testing.T) {
	testutil.TestAcquireTwiceForSameFamily(t, createTestNameManager(t, 0))
}

func TestAcquireForDifferentFamilies(t *testing.T) {
	testutil.TestAcquireForDifferentFamilies(t, createTestNameManager(t, 0))
}

func TestAcquireReleaseThenAcquireForAnotherFamily(t *testing.T) {
	testutil.TestAcquireReleaseThenAcquireForAnotherFamily(t, createTestNameManager(t, 0))
}

func TestAcquireAcquireReleaseAcquireAcquire(t *testing.T) {
	testutil.TestAcquireAcquireReleaseAcquireAcquire(t, createTestNameManager(t, 0))
}

func TestTryAcquire(t *testing.T) {
	testutil.TestTryAcquire(t, createTestNameManager(t, 0))
}

func TestTryAcquireErrors(t *testing.T) {
	testutil.TestTryAcquireErrors(t, createTestNameManager(t, 0))
}

func createTestNameManager(t *testing.T, autoReleaseAfter int) name_manager.NameManager {
	ts, err := testserver.New(autoReleaseAfter)
	assert.Nil(t, err)

	url := "localhost:%d"
	if autoReleaseAfter > 0 {
		url = url + fmt.Sprintf(";keepAliveInterval=%ds", autoReleaseAfter/3)
	}
	manager, err := createNameManager(fmt.Sprintf(url, ts.Port))
	assert.Nil(t, err)

	manager.(*restBackend).resetHook = ts.Clean
	return manager
}
