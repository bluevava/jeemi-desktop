package helperstate

import "errors"

// Replace verifies staged bytes before stopping the old service. Any failure
// after promotion attempts rollback, and rollback failure remains an error.
func Replace(prepare, stop, promote, verify, rollback func() error) error {
	if err := prepare(); err != nil {
		return err
	}
	if err := stop(); err != nil {
		return err
	}
	if err := promote(); err != nil {
		return errors.Join(err, rollback())
	}
	if err := verify(); err != nil {
		return errors.Join(err, rollback())
	}
	return nil
}
