// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package test

import (
	"fmt"
	"github.com/benbjohnson/clock"
	"github.com/hchauvin/name_manager/pkg/local_backend"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/hchauvin/name_manager/pkg/server"
	"io/ioutil"
	"net"
)

type TestServer struct {
	Port  int
	Impl  name_manager.NameManager
	Clean func()
}

func New(autoReleaseAfter int) (*TestServer, error) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		return nil, err
	}

	implURL := "local://" + tmpfile.Name()
	if autoReleaseAfter > 0 {
		implURL = implURL + fmt.Sprintf(";autoReleaseAfter=%ds", autoReleaseAfter)
	}

	manager, err := name_manager.CreateFromURL(implURL)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		manager.Reset()
		return nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	go func() {
		server.Serve(listener, manager)
	}()

	return &TestServer{
		Port: port,
		Impl: manager,
		Clean: func() {
			listener.Close()
			manager.Reset()
		},
	}, nil
}

func (s *TestServer) MockClock(c clock.Clock) {
	local_backend.MockClock(s.Impl, c)
}
