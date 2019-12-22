// Package hold implement the holding mechanism common to all the backends.
package hold

import (
	"errors"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"os"
	"time"
)

type Hold struct {
	Manager name_manager.NameManager
	// clock is the clock used to get the CreatedAt/UpdatedAt timestamps.
	Clock             clock.Clock
	KeepAliveInterval time.Duration
}

func (h *Hold) Hold(family string) (string, <-chan error, name_manager.ReleaseFunc, error) {
	name, err := h.Manager.Acquire(family)
	if err != nil {
		return "", nil, nil, err
	}

	errc, releaseFunc, err := h.holdCommon(family, name)
	if err != nil {
		return "", nil, nil, err
	}
	return name, errc, releaseFunc, nil
}

func (h *Hold) TryHold(family, name string) (<-chan error, name_manager.ReleaseFunc, error) {
	if err := h.Manager.TryAcquire(family, name); err != nil {
		return nil, nil, err
	}

	errc, releaseFunc, err := h.holdCommon(family, name)
	if err != nil {
		return nil, nil, err
	}
	return errc, releaseFunc, nil
}

func (h *Hold) holdCommon(family, name string) (<-chan error, name_manager.ReleaseFunc, error) {
	errc := make(chan error, 1)

	var stopKeepAlive, keepAliveDone chan struct{}
	if h.KeepAliveInterval > 0 {
		stopKeepAlive = make(chan struct{})
		keepAliveDone = make(chan struct{})
		go func() {
			defer close(keepAliveDone)

			for {
				select {
				case <-stopKeepAlive:
					return
				case <-h.Clock.After(h.KeepAliveInterval):
				}

				if err := retry.Do(func() error {
					return h.Manager.KeepAlive(family, name)
				}, retry.Delay(200*time.Millisecond), retry.Attempts(3)); err != nil {
					msg := fmt.Sprintf("cannot keep alive %s:%s: %v\n", family, name, err)
					fmt.Fprintf(os.Stderr, msg)
					errc <- errors.New(msg)
					return
				}
			}
		}()
	}

	releaseFunc := func() error {
		if h.KeepAliveInterval > 0 {
			close(stopKeepAlive)
			<-keepAliveDone
		}
		close(errc)
		if err := h.Manager.Release(family, name); err != nil {
			return err
		}
		return nil
	}

	return errc, releaseFunc, nil
}
