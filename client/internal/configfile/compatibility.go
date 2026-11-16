package configfile

import "errors"

// CompatibilityError is not corruption and must never authorize repair/deletion.
type CompatibilityError struct{ message string }

func (e *CompatibilityError) Error() string  { return e.message }
func UnsupportedFormat(message string) error { return &CompatibilityError{message: message} }
func IsUnsupported(err error) bool {
	var unsupported *CompatibilityError
	return errors.As(err, &unsupported)
}

func MigrationFailed(path string, cause error) error {
	return &Failure{Path: path, Reason: "migration", cause: cause}
}
