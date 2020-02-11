// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package firestore_backend

import (
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/hchauvin/name_manager/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"os"
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
	testutil.TestList(t, mng, nil)
}

func TestKeepAlive(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	testutil.TestKeepAlive(t, mng, nil)
}

func TestHold(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	testutil.TestHold(t, mng, nil)
}

func TestTryAcquire(t *testing.T) {
	testutil.TestTryAcquire(t, createTestNameManager(t))
}

func TestTryAcquireErrors(t *testing.T) {
	testutil.TestTryAcquireErrors(t, createTestNameManager(t))
}

func TestTryHold(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	testutil.TestTryHold(t, mng, nil)
}

func createTestNameManager(t *testing.T, options ...string) name_manager.NameManager {
	// os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080")
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("No Firestore")
		return nil
	}

	var url strings.Builder
	url.WriteString("projectID=project")
	for _, option := range options {
		url.WriteRune(';')
		url.WriteString(option)
	}
	manager, err := createNameManager(url.String())
	assert.Nil(t, err)
	err = manager.Reset()
	assert.Nil(t, err)
	return manager
}
