// Package defgen holds code-generation helpers. It is never imported by runtime
// code, but must contain at least one buildable file so `go list ./...` works.
package defgen

// This file ensures the package is buildable for tools like `go list`.
