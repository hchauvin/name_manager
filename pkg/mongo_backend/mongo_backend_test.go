package mongo_backend

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/benbjohnson/clock"
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
	mockClock := clock.NewMock()
	mng.(*mongoBackend).clock = mockClock
	testutil.TestList(t, mng, mockClock)
}

func TestKeepAlive(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	mockClock := clock.NewMock()
	mng.(*mongoBackend).clock = mockClock
	testutil.TestKeepAlive(t, mng, mockClock)
}

func TestHold(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	mockClock := clock.NewMock()
	mng.(*mongoBackend).clock = mockClock
	testutil.TestHold(t, mng, mockClock)
}

func TestTryAcquire(t *testing.T) {
	testutil.TestTryAcquire(t, createTestNameManager(t))
}

func TestTryAcquireErrors(t *testing.T) {
	testutil.TestTryAcquireErrors(t, createTestNameManager(t))
}

func TestTryHold(t *testing.T) {
	mng := createTestNameManager(t, "autoReleaseAfter=5s")
	mockClock := clock.NewMock()
	mng.(*mongoBackend).clock = mockClock
	testutil.TestTryHold(t, mng, mockClock)
}

func createTestNameManager(t *testing.T, options ...string) name_manager.NameManager {
	uri := os.Getenv("MONGODB_URI")
	// uri := "mongodb://127.0.0.1:27017"
	if uri == "" {
		t.Skip("No MongoDB")
		return nil
	}
	db, err := randomHex(10)
	if err != nil {
		panic(err)
	}

	var url strings.Builder
	url.WriteString("uri=" + uri + ";database=" + db + ";collectionPrefix=nm_")
	for _, option := range options {
		url.WriteRune(';')
		url.WriteString(option)
	}
	manager, err := createNameManager(url.String())
	assert.Nil(t, err)
	return manager
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
