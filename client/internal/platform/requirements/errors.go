// Package requirements checks platform prerequisites without changing system
// networking or requesting elevated privileges.
package requirements

import "errors"

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func Code(err error, fallback string) string {
	var platformError *Error
	if errors.As(err, &platformError) {
		return platformError.Code
	}
	return fallback
}
