package local_backend

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type options struct {
	autoReleaseAfter time.Duration
}

func parseBackendURL(backendURL string) (string, *options, error) {
	components := strings.Split(backendURL, ";")

	path, err := expandHome(components[0])
	if err != nil {
		return "", nil, err
	}

	options, err := parseOptions(components[1:])
	if err != nil {
		return "", nil, err
	}

	return path, options, nil
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
	return opts, nil
}
