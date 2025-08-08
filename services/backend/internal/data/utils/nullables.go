// Package utils provides utility functions for data handling, time operations,
// and other common helper functionality used throughout the application.
//
// revive:disable:var-naming - package name 'utils' is conventional in Go projects
package utils

// NullString returns empty string if input is empty/zero value
func NullString(s string) string {
	if s == "" {
		return ""
	}
	return s
}

// NullInt64 returns 0 if input is zero value
func NullInt64(i int64) int64 {
	if i == 0 {
		return 0
	}
	return i
}

// revive:enable:var-naming
