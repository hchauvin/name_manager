// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package name_manager

import (
	"fmt"
	"time"
)

// NameManager objects are responsible for the acquisition and release
// of names with a global lock.
type NameManager interface {
	// Hold acquires a name for the given family, returns it, and keep
	// it alive until the release function is called (this call also
	// releases the name).  Thanks to a global lock, a given name cannot
	// be acquired twice for the same family without having been released
	// first.  Using Hold in the background is almost always preferable
	// to calling directly Acquire, KeepAlive, and Release, which are
	// low-level.
	Hold(family string) (string, ReleaseFunc, error)

	// Acquire acquires a name for the given family, and returns it.
	// Thanks to a global lock, a given name cannot be acquired twice for
	// the same family without having been released first.  After being
	// acquired, a name must be kept alive, otherwise it can automatically
	// released after a certain time.  The automatic release is
	// backend-specific.
	Acquire(family string) (string, error)

	// KeepAlive produces a heart beat to avoid a name being automatically
	// released after a certain time.  KeepAlive helps to avoid zombies.
	// Note that automatic release does NOT have to be implemented by a
	// backend.
	KeepAlive(family, name string) error

	// Release releases a name previously registered for a family.
	// It is not an error to release a name that has already been released,
	// or that was never acquired in the first place.  A name that has
	// be released can be acquired again.
	Release(family, name string) error

	// List lists the names that are currently registered, either marked as
	// `free` or not.
	List() ([]Name, error)

	// Reset deregister all the names.  After this call, `List` returns
	// `nil`.
	Reset() error
}

// ReleaseFunc is called to release a name that was acquired and kept
// alive through `NameManager.Hold`.
type ReleaseFunc func() error

// Name describes a name as registered with a `NameManager`.
type Name struct {
	// Name is the name.
	Name string

	// Family is the name family the name belongs to.  Names are unique
	// within the same family.

	Family string

	// CreatedAt is the timestamp at which the name was first registered.
	CreatedAt time.Time

	// UpdatedAt is the timestamp at which the name was last acquired.
	UpdatedAt time.Time

	// Free is whether the name is free, or it was acquired but not
	// yet released.
	Free bool
}

// Backend describes a backend for creating name managers.
type Backend struct {
	// Protocol is the protocol for the backend.  If the protocol is "foo",
	// backend URLs starting with "foo://", such as "foo://bar", will give
	// `NameManager` instances created with the `CreateNameManager`
	// associated with this backend.
	Protocol string

	// Description holds a human-readable description of the backend.
	// This description should specify, among other things, the format
	// for the URL.
	Description string

	// CreateNameManager creates a `NameManager` from a URL, stripped
	// of the protocol.  For instance, if `CreateFromURL("foo://bar")` is
	// called, the URL passed to this function is "bar".
	CreateNameManager func(backendURL string) (NameManager, error)
}

// backends holds the list of backends registered with `RegisterBackend`.
var backends = make(map[string]Backend)

// RegisterBackend registers a backend.  Backends cannot be used with
// CreateFromURL unless they are registered with this function.  This function
// should be called in the `init` function of the backend packages.
func RegisterBackend(backend Backend) {
	if _, ok := backends[backend.Protocol]; ok {
		panic(fmt.Sprintf("backend '%s' is already registered", backend.Protocol))
	}
	backends[backend.Protocol] = backend
}

// CreateFromURL creates a `NameManager` from a url.  The URL, e.g., "foo://bar",
// contains a backend protocol, e.g., "foo", and a backend-specific URL, e.g.,
// "bar".
func CreateFromURL(url string) (NameManager, error) {
	backendProtocol, backendURL, err := parseURL(url)
	if err != nil {
		return nil, err
	}

	backend, ok := backends[backendProtocol]
	if !ok {
		return nil, fmt.Errorf("backend '%s' has not been registered", backendProtocol)
	}

	return backend.CreateNameManager(backendURL)
}
