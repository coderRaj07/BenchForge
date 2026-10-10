// Package redact removes credentials from strings that may be logged.
package redact

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	passwordKV = regexp.MustCompile(`(?i)(password\s*=\s*)('[^']*'|\S+)`)
	envSecret  = regexp.MustCompile(`(?i)((?:PGPASSWORD|POSTGRES_PASSWORD)=)\S+`)
)

// DSN returns a copy of a Postgres URL or keyword DSN with the password removed.
func DSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		u, err := url.Parse(trimmed)
		if err == nil && u.User != nil {
			user := u.User.Username()
			u.User = url.UserPassword(user, "***")
			return u.String()
		}
	}
	return passwordKV.ReplaceAllString(trimmed, `${1}***`)
}

// Secrets redacts well-known password environment assignments.
func Secrets(s string) string {
	return envSecret.ReplaceAllString(s, `${1}***`)
}
