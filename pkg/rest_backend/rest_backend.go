// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package rest_backend

import (
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/internal/hold"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"io/ioutil"
	"net/http"
	"strings"
)

var backendDescription = `REST backend.

The REST backend communicates with a name_manager server.
`

func init() {
	name_manager.RegisterBackend(name_manager.Backend{
		Protocol:          "rest",
		Description:       backendDescription,
		CreateNameManager: createNameManager,
	})
}

func createNameManager(backendURL string) (name_manager.NameManager, error) {
	url, options, err := parseBackendURL(backendURL)
	if err != nil {
		return nil, err
	}
	return &restBackend{
		url:     url,
		clock:   clock.New(),
		options: *options,
		client:  http.Client{},
	}, nil
}

type restBackend struct {
	// url is the base URL for the REST server.
	url string
	// clock is the clock used to get the CreatedAt/UpdatedAt timestamps.
	clock clock.Clock
	// options are the options for the backend.
	options options
	// client is the HTTP client to use to communicate with the REST server.
	client http.Client
	// resetHook is an optional hook that is called by Reset.
	// It is used for testing.
	resetHook func()
}

func (rbk *restBackend) Hold(family string) (string, <-chan error, name_manager.ReleaseFunc, error) {
	return rbk.hold().Hold(family)
}

func (rbk *restBackend) Acquire(family string) (string, error) {
	return rbk.get(fmt.Sprintf("/family/%s/$acquire", family))
}

func (rbk *restBackend) KeepAlive(family, name string) error {
	_, err := rbk.get(fmt.Sprintf("/family/%s/name/%s/$keep_alive", family, name))
	return err
}

func (rbk *restBackend) Release(family, name string) error {
	_, err := rbk.get(fmt.Sprintf("/family/%s/name/%s/$release", family, name))
	return err
}

func (rbk *restBackend) TryHold(family, name string) (<-chan error, name_manager.ReleaseFunc, error) {
	return rbk.hold().TryHold(family, name)
}

func (rbk *restBackend) TryAcquire(family, name string) error {
	body, err := rbk.get(fmt.Sprintf("/family/%s/name/%s/$try_acquire", family, name))
	if err != nil {
		return err
	}
	if body == "ERR_NOT_EXIST" {
		return name_manager.ErrNotExist
	}
	if body == "ERR_IN_USE" {
		return name_manager.ErrInUse
	}
	return nil
}

func (rbk *restBackend) List() ([]name_manager.Name, error) {
	body, err := rbk.get("/")
	if err != nil {
		return nil, err
	}
	var names []name_manager.Name
	if err := json.Unmarshal([]byte(body), &names); err != nil {
		return nil, err
	}
	return names, nil
}

func (rbk *restBackend) Reset() error {
	if rbk.resetHook != nil {
		rbk.resetHook()
	}
	_, err := rbk.get("/$reset")
	return err
}

func (rbk *restBackend) get(endpoint string) (string, error) {
	var body string
	err := func() error {
		resp, err := rbk.client.Get(rbk.url + endpoint)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("%s: non-200 status code: %s", endpoint, resp.Status)
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return retry.Unrecoverable(err)
		}
		body = strings.TrimSpace(string(b))
		return nil
	}()
	if err != nil {
		return "", err
	}
	return body, nil
}

func (rbk *restBackend) hold() *hold.Hold {
	return &hold.Hold{
		Manager:           rbk,
		Clock:             rbk.clock,
		KeepAliveInterval: rbk.options.keepAliveInterval,
	}
}
