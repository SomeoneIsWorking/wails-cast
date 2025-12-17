package urlhelper

import (
	"fmt"
	"net/url"
)

func UPrintf(format string, args ...any) *url.URL {
	return Parse(fmt.Sprintf(format, args...))
}

func Parse(rawurl string) *url.URL {
	url, err := url.Parse(rawurl)
	if err != nil {
		return nil
	}
	return url
}
