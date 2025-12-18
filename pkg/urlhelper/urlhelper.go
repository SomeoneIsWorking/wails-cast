package urlhelper

import (
	"fmt"
	"net/url"
)

func UPrintf(format string, args ...any) *url.URL {
	return ParseFixed(fmt.Sprintf(format, args...))
}

func ParseFixed(rawurl string) *url.URL {
	url, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	return url
}
