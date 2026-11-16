package configworkspace

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type fileRecord struct {
	info   fs.FileInfo
	digest [32]byte
}

func safePath(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || (rel != "." && !filepath.IsLocal(rel)) {
		return fmt.Errorf("configuration path escapes its root")
	}
	current := filepath.Clean(root)
	parts := []string{""}
	if rel != "." {
		parts = append(parts, strings.Split(rel, string(filepath.Separator))...)
	}
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if unsafeFile(info) || (i < len(parts)-1 && !info.IsDir()) {
			return fmt.Errorf("configuration path is not a regular managed path")
		}
	}
	return nil
}

func inventory(ctx context.Context, source string) ([]string, error) {
	var files []string
	for _, name := range configurationEntries {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := safePath(source, path); err != nil {
			return nil, err
		}
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if unsafeFile(info) {
				return fmt.Errorf("configuration tree contains an unsupported file")
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func copyConfiguration(ctx context.Context, source, target string, copyFile func(context.Context, string, string) (fileRecord, error)) (map[string]fileRecord, error) {
	files, err := inventory(ctx, source)
	if err != nil {
		return nil, err
	}
	records := make(map[string]fileRecord, len(files))
	for _, path := range files {
		from, to := filepath.Join(source, path), filepath.Join(target, path)
		if err := safePath(source, from); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			return nil, err
		}
		record, err := copyFile(ctx, from, to)
		if err != nil {
			return nil, err
		}
		records[path] = record
	}
	return records, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func copyRegularFile(ctx context.Context, source, target string) (fileRecord, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return fileRecord{}, err
	}
	if unsafeFile(info) || !info.Mode().IsRegular() {
		return fileRecord{}, fmt.Errorf("configuration source is not a regular file")
	}
	in, err := os.Open(source)
	if err != nil {
		return fileRecord{}, err
	}
	defer in.Close()
	opened, err := in.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return fileRecord{}, fmt.Errorf("configuration source changed")
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fileRecord{}, err
	}
	defer out.Close()
	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, digest), contextReader{ctx, in})
	if err != nil {
		return fileRecord{}, err
	}
	if written != info.Size() {
		return fileRecord{}, fmt.Errorf("configuration source size changed")
	}
	if err := out.Sync(); err != nil {
		return fileRecord{}, err
	}
	if err := out.Close(); err != nil {
		return fileRecord{}, err
	}
	record := fileRecord{info: info}
	copy(record.digest[:], digest.Sum(nil))
	return record, nil
}

func verifySource(ctx context.Context, source string, records map[string]fileRecord) error {
	files, err := inventory(ctx, source)
	if err != nil {
		return err
	}
	if len(files) != len(records) {
		return fmt.Errorf("configuration source changed during migration")
	}
	for _, path := range files {
		record, ok := records[path]
		if !ok {
			return fmt.Errorf("configuration source changed during migration")
		}
		full := filepath.Join(source, path)
		info, err := os.Lstat(full)
		if err != nil || unsafeFile(info) || !os.SameFile(info, record.info) || info.Size() != record.info.Size() || !info.ModTime().Equal(record.info.ModTime()) {
			return fmt.Errorf("configuration source changed during migration")
		}
		file, err := os.Open(full)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, contextReader{ctx, file})
		file.Close()
		if err != nil {
			return err
		}
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		if digest != record.digest {
			return fmt.Errorf("configuration source changed during migration")
		}
	}
	return nil
}

// Flush transformed files as well as the directory entries that name them.
// Synchronizing only the top directory does not persist nested directories.
func syncWorkspace(ctx context.Context, root string) error {
	var directories []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if unsafeFile(info) {
			return fmt.Errorf("migration produced an unsafe file")
		}
		if entry.IsDir() {
			directories = append(directories, path)
			return nil
		}
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		err = file.Sync()
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	}); err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := syncDirectory(directories[i]); err != nil {
			return err
		}
	}
	return nil
}
