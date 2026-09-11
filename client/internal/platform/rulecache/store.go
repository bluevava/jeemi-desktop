package rulecache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
)

const maxFileBytes int64 = 256 << 20
const maxSessionBytes int64 = 1 << 30

// Access runs filesystem operations as the cache-owning account. Windows
// retains the authenticated caller's token; macOS uses its protected UID cache.
type Access func(func() error) error

type Store struct {
	Root   string
	Path   string
	Access Access
}

func (s Store) access(action func() error) error {
	if s.Access != nil {
		return s.Access(action)
	}
	return action()
}

func (s Store) open(name string) (file *os.File, err error) {
	err = s.access(func() error {
		root, e := os.OpenRoot(s.Root)
		if e != nil {
			return e
		}
		defer root.Close()
		file, e = openRegular(root, path.Join(s.Path, name))
		return e
	})
	return
}

func (s Store) save(ctx context.Context, name string, source *os.File, info fs.FileInfo) error {
	return s.access(func() error {
		root, err := os.OpenRoot(s.Root)
		if err != nil {
			return err
		}
		defer root.Close()
		if err = makeDirectories(root, s.Path); err != nil {
			return err
		}
		target := path.Join(s.Path, name)
		if previous, e := openRegular(root, target); e == nil {
			old, statErr := previous.Stat()
			previous.Close()
			if statErr == nil && old.Size() == info.Size() && old.ModTime().Equal(info.ModTime()) {
				return nil
			}
		}
		return atomicCopy(root, target, contextReader{ctx, source}, info.Size(), info.ModTime())
	})
}

type contextReader struct {
	ctx context.Context
	io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(buffer)
}

func openRegular(root *os.Root, name string) (*os.File, error) {
	if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
		return nil, errInvalid
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) || (i == len(parts)-1 && (!info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxFileBytes)) {
			return nil, errInvalid
		}
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxFileBytes {
		file.Close()
		return nil, errInvalid
	}
	return file, nil
}

func makeDirectories(root *os.Root, name string) error {
	if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
		return errInvalid
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		if err := root.Mkdir(current, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := root.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errInvalid
		}
	}
	return nil
}

// Write to a sibling and rename only after a complete, unchanged read. Never
// replace a valid cache with a truncated file, and retain mihomo's expiry time.
func atomicCopy(root *os.Root, target string, source io.Reader, size int64, modified time.Time) error {
	if !fs.ValidPath(target) || size < 0 || size > maxFileBytes {
		return errInvalid
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temporary := target + ".incoming-" + hex.EncodeToString(nonce[:])
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	_, err = io.CopyN(file, source, size)
	if err == nil {
		var extra [1]byte
		n, e := source.Read(extra[:])
		if n != 0 || e != io.EOF {
			err = errInvalid
		}
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if !modified.IsZero() {
		if err = root.Chtimes(temporary, modified, modified); err != nil {
			return err
		}
	}
	return root.Rename(temporary, target)
}
