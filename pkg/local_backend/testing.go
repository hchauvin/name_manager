package local_backend

import (
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
)

func MockClock(manager name_manager.NameManager, c clock.Clock) {
	manager.(*localBackend).clock = c
}
