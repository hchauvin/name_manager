package name_manager

import (
	"fmt"
	"regexp"
)

var parseURLRe = regexp.MustCompile("^([^:]+)://(.+)$")

func parseURL(url string) (backendProtocol string, backendURL string, err error) {
	submatches := parseURLRe.FindStringSubmatch(url)
	if submatches == nil {
		return "", "", fmt.Errorf("name manager: invalid backend URL: '%s'", url)
	}
	if len(submatches) != 3 {
		panic("expected 3 submatches")
	}
	backendProtocol = submatches[1]
	backendURL = submatches[2]
	return
}
