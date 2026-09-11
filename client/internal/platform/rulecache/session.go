package rulecache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Session struct {
	workspace string
	store     Store
	keys      map[string]bool
}

func NewSession(workspace string, store Store) *Session {
	return &Session{workspace: workspace, store: store, keys: map[string]bool{}}
}

// Prepare handles only the received generation, keeping live cache files from
// earlier reloads. Rewritten paths are private to the helper's runtime copy.
func (s *Session) Prepare(configuration string) error {
	root, err := os.OpenRoot(s.workspace)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := openRegular(root, configuration)
	if err != nil {
		return err
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxConfigurationBytes+1))
	file.Close()
	if err != nil {
		return err
	}
	updated, entries, err := rewrite(contents)
	if err != nil || len(entries) == 0 {
		return err
	}
	if err = makeDirectories(root, sessionDirectory); err != nil {
		return err
	}
	var total int64
	for _, item := range entries {
		s.keys[item.key] = true
		name := path.Join(sessionDirectory, item.key)
		if current, e := openRegular(root, name); e == nil {
			current.Close()
			continue
		}
		source, e := s.store.open(item.key)
		// Honour an explicitly supplied existing HTTP provider file on the first
		// run as well. The snapshot already owns these independent input bytes.
		if e != nil && item.previous != "" {
			previous := item.previous
			if filepath.IsAbs(previous) {
				previous, _ = filepath.Rel(s.workspace, previous)
			}
			previous = filepath.ToSlash(previous)
			previous = strings.TrimPrefix(previous, "./")
			// Windows paths are case-insensitive. Never seed a provider from a
			// generation or database, even through a differently cased spelling.
			namespace := strings.ToLower(strings.SplitN(previous, "/", 2)[0])
			if namespace != "generations" && namespace != "resolved" && namespace != "cache.db" && !strings.HasPrefix(previous, ".") {
				source, e = openRegular(root, previous)
			}
		}
		if e != nil {
			continue // Missing/unusable cache remains a normal cold download.
		}
		info, e := source.Stat()
		if e == nil {
			total += info.Size()
			if total <= maxSessionBytes {
				e = atomicCopy(root, name, source, info.Size(), info.ModTime())
			}
		}
		source.Close()
		if e != nil || total > maxSessionBytes {
			log.Print("Jeemi: rule-provider cache restore skipped")
		}
	}
	return atomicCopy(root, configuration, bytes.NewReader(updated), int64(len(updated)), time.Time{})
}

// Save runs after the core has exited and before its private session is erased.
// It copies only known HTTP provider files, never cache.db or session secrets.
func (s *Session) Save(ctx context.Context) error {
	root, err := os.OpenRoot(s.workspace)
	if err != nil {
		return err
	}
	defer root.Close()
	var total int64
	var result error
	for key := range s.keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, e := openRegular(root, path.Join(sessionDirectory, key))
		if e != nil {
			if !errors.Is(e, os.ErrNotExist) {
				result = errInvalid
			}
			continue
		}
		info, e := file.Stat()
		if e == nil {
			total += info.Size()
			if total > maxSessionBytes {
				e = errInvalid
			} else {
				e = s.store.save(ctx, key, file, info)
			}
		}
		file.Close()
		if e != nil {
			result = errInvalid
		}
	}
	return result
}
