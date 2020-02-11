// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package firestore_backend

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type options struct {
	projectID        string
	prefix           string
	autoReleaseAfter time.Duration
}

func parseBackendURL(backendURL string) (*options, error) {
	components := strings.Split(backendURL, ";")

	opts := &options{}
	for _, s := range components {
		components := strings.SplitN(s, "=", 2)
		if len(components) != 2 {
			return nil, errors.New("URL format error: options must have format \"key=value\"")
		}
		key := components[0]
		value := components[1]
		switch key {
		case "projectID":
			opts.projectID = value

		case "prefix":
			opts.prefix = value

		case "autoReleaseAfter":
			var err error
			opts.autoReleaseAfter, err = time.ParseDuration(value)
			if err != nil {
				return nil, fmt.Errorf("cannot parse duration for autoReleaseAfter: %v", err)
			}

		default:
			return nil, fmt.Errorf("unrecognized option \"%s\"", key)
		}
	}

	if opts.projectID == "" {
		return nil, errors.New("projectID option is mandatory")
	}

	return opts, nil
}
