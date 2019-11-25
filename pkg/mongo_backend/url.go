package mongo_backend

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type options struct {
	uri string
	database string
	collectionPrefix string
	autoReleaseAfter time.Duration
	variant variant
}

type variant int

const (
	genericVariant variant = iota
	cosmosDBVariant
)

var stringToVariant = map[string]variant{
	"generic": genericVariant,
	"cosmosDB": cosmosDBVariant,
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
		case "uri":
			opts.uri = value

		case "database":
			opts.database = value

		case "collectionPrefix":
			opts.collectionPrefix = value

		case "autoReleaseAfter":
			var err error
			opts.autoReleaseAfter, err = time.ParseDuration(value)
			if err != nil {
				return nil, fmt.Errorf("cannot parse duration for autoReleaseAfter: %v", err)
			}

		case "variant":
			variant, ok := stringToVariant[value]
			if !ok {
				return nil, fmt.Errorf("unrecognized variant '%s'", value)
			}
			opts.variant = variant

		default:
			return nil, fmt.Errorf("unrecognized option \"%s\"", key)
		}
	}

	if opts.uri == "" {
		return nil, errors.New("uri option is mandatory")
	}
	if opts.database == "" {
		return nil, errors.New("database option is mandatory")
	}
	if opts.collectionPrefix == "" {
		return nil, errors.New("collectionPrefix option is mandatory")
	}

	return opts, nil
}
