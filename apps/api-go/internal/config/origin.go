package config

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateOrigin is called only by the HTTP process; migrations do not serve cookies.
func ValidateOrigin(origin string, localHTTP bool) error {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || strings.HasSuffix(origin, "/") {
		return errors.New("APP_ORIGIN must be an exact origin without a trailing slash, path, query or credentials")
	}
	if localHTTP {
		if u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
			return errors.New("ALLOW_LOCAL_HTTP is limited to an http:// loopback APP_ORIGIN")
		}
	} else if u.Scheme != "https" {
		return errors.New("APP_ORIGIN must use HTTPS; local development requires explicit ALLOW_LOCAL_HTTP")
	}
	return nil
}
