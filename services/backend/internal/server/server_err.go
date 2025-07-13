package server

import (
	"errors"
	"net/http"
)

// Sentinel (application) errors that can be wrapped and checked with errors.Is.
// Extend this list gradually as new public-facing error classes are needed.
var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrIncorrectEmail     = errors.New("incorrect email")
	ErrIncorrectPassword  = errors.New("incorrect password")
	ErrGoogleAuthRequired = errors.New("google auth required")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrUsageExceeded      = errors.New("usage limit exceeded")
)

// appErrorInfo associates a sentinel error with an HTTP status code and a
// short, safe message that can be sent to the client.
type appErrorInfo struct {
	statusCode int
	publicMsg  string
}

// Mapping table from sentinel to HTTP metadata.
var appErrorTable = map[error]appErrorInfo{
	ErrInvalidInput:       {http.StatusBadRequest, "Invalid input"},
	ErrUnauthorized:       {http.StatusUnauthorized, "Unauthorized"},
	ErrNotFound:           {http.StatusNotFound, "Not found"},
	ErrConflict:           {http.StatusConflict, "Conflict"},
	ErrEmailExists:        {http.StatusBadRequest, "Email already registered"},
	ErrIncorrectEmail:     {http.StatusUnauthorized, "Incorrect email"},
	ErrIncorrectPassword:  {http.StatusUnauthorized, "Incorrect password"},
	ErrGoogleAuthRequired: {http.StatusUnauthorized, "This account uses Google Sign-In. Please login with Google."},
	ErrInvalidCredentials: {http.StatusUnauthorized, "Invalid credentials"},
	ErrInsufficientFunds:  {http.StatusPaymentRequired, "Insufficient credits or funds"},
	ErrUsageExceeded:      {http.StatusTooManyRequests, "Usage limit exceeded"},
}

// resolveAppError converts an error (possibly wrapped) to an HTTP status code
// and a public-facing message. If the error does not match any sentinel, a
// generic 500 response is returned.
func resolveAppError(err error) (int, string) {
	for sentinel, info := range appErrorTable {
		if errors.Is(err, sentinel) {
			return info.statusCode, info.publicMsg
		}
	}
	return http.StatusInternalServerError, "Unexpected error"
}
