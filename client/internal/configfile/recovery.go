// Package configfile identifies startup configuration failures without exposing
// document contents, and deletes only a file the user has just confirmed.
package configfile

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrDeclined = errors.New("startup configuration recovery declined")

type Failure struct {
	Path   string
	Reason string
	Offset int64
	cause  error
	digest [sha256.Size]byte
	size   int64
}

func (f *Failure) Error() string { return f.cause.Error() }
func (f *Failure) Unwrap() error { return f.cause }

func Invalid(path string, contents []byte, cause error) error {
	f := &Failure{Path: path, Reason: "invalid", cause: cause, digest: sha256.Sum256(contents), size: int64(len(contents))}
	var syntax *json.SyntaxError
	var value *json.UnmarshalTypeError
	if errors.As(cause, &syntax) {
		f.Offset = syntax.Offset
	} else if errors.As(cause, &value) {
		f.Offset = value.Offset
	}
	return f
}

func Unreadable(path string, cause error) error {
	return &Failure{Path: path, Reason: "read", cause: cause}
}

// Recover repeats the real startup check after each individual confirmation.
// Declining, cancelling, or an IO failure never authorizes deleting another file.
func Recover(ctx context.Context, root string, check func() error, confirm func(*Failure) (bool, error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := check()
		if err == nil {
			return ctx.Err()
		}
		var failure *Failure
		if !errors.As(err, &failure) || failure.Reason != "invalid" {
			return err
		}
		if err := managedFile(root, failure.Path); err != nil {
			return Unreadable(failure.Path, err)
		}
		accepted, err := confirm(failure)
		if err != nil {
			return err
		}
		if !accepted {
			return ErrDeclined
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := removeConfirmed(root, failure); err != nil {
			return &Failure{Path: failure.Path, Reason: "delete", cause: err}
		}
	}
}

func managedFile(root, path string) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return fmt.Errorf("configuration path must be absolute")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return fmt.Errorf("configuration path is outside the data directory")
	}
	current := filepath.Clean(root)
	parts := append([]string{""}, strings.Split(relative, string(filepath.Separator))...)
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (index < len(parts)-1 && !info.IsDir()) || (index == len(parts)-1 && !info.Mode().IsRegular()) {
			return fmt.Errorf("configuration path is not a regular managed file")
		}
	}
	return nil
}

func removeConfirmed(root string, failure *Failure) error {
	if err := managedFile(root, failure.Path); err != nil {
		return err
	}
	file, err := os.Open(failure.Path)
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	hash := sha256.New()
	size, readErr := io.Copy(hash, io.LimitReader(file, failure.size+1))
	closeErr := file.Close()
	if statErr != nil || readErr != nil || closeErr != nil {
		return fmt.Errorf("could not recheck the confirmed configuration file")
	}
	if size != failure.size || string(hash.Sum(nil)) != string(failure.digest[:]) {
		return fmt.Errorf("configuration file changed after confirmation")
	}
	if err := managedFile(root, failure.Path); err != nil {
		return err
	}
	current, err := os.Lstat(failure.Path)
	if err != nil || !os.SameFile(info, current) {
		return fmt.Errorf("configuration file was replaced after confirmation")
	}
	return os.Remove(failure.Path)
}
