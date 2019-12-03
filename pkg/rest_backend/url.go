// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package rest_backend

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type options struct {
	keepAliveInterval time.Duration
}

func parseBackendURL(backendURL string) (string, *options, error) {
	components := strings.Split(backendURL, ";")
	url := "http://" + components[0]
	options, err := parseOptions(components[1:])
	if err != nil {
		return "", nil, err
	}

	return url, options, nil
}

func parseOptions(str []string) (*options, error) {
	opts := &options{}
	for _, s := range str {
		components := strings.SplitN(s, "=", 2)
		if len(components) != 2 {
			return nil, errors.New("URL format error: options must have format \"key=value\"")
		}
		key := components[0]
		value := components[1]
		switch key {
		case "keepAliveInterval":
			var err error
			opts.keepAliveInterval, err = time.ParseDuration(value)
			if err != nil {
				return nil, fmt.Errorf("cannot parse duration for keepAliveInterval: %v", err)
			}

		default:
			return nil, fmt.Errorf("unrecognized option \"%s\"", key)
		}
	}
	return opts, nil
}
