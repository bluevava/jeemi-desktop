package externalui

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxExpandedBytes int64 = 256 << 20
const maxFiles = 4096

type metadata struct {
	Release Release          `json:"release"`
	Files   map[string]int64 `json:"files"`
}

func (m *Manager) Installed(version string) (Release, error) {
	return m.installed(version, true)
}

func (m *Manager) installed(version string, full bool) (Release, error) {
	if !ValidVersion(version) {
		return Release{}, fmt.Errorf("invalid zashboard version")
	}
	root, err := os.OpenRoot(m.root)
	if err != nil {
		return Release{}, fmt.Errorf("zashboard resources are not installed")
	}
	defer root.Close()
	info, err := root.Lstat(version)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Release{}, fmt.Errorf("zashboard resources are not installed")
	}
	file, err := root.Open(version + "/metadata.json")
	if err != nil {
		return Release{}, fmt.Errorf("zashboard metadata is unavailable")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	var meta metadata
	if err != nil || len(data) > 1<<20 || json.Unmarshal(data, &meta) != nil || meta.Release.Version != version || len(meta.Files) == 0 || len(meta.Files) > maxFiles {
		return Release{}, fmt.Errorf("invalid zashboard metadata")
	}
	if meta.Files["index.html"] <= 0 {
		return Release{}, fmt.Errorf("zashboard index is missing")
	}
	for name, size := range meta.Files {
		if !safeName(name) || size < 0 || size > maxExpandedBytes {
			return Release{}, fmt.Errorf("invalid zashboard metadata")
		}
		if !full && name != "index.html" {
			continue
		}
		info, err := root.Lstat(version + "/dist/" + name)
		if err != nil || !info.Mode().IsRegular() || info.Size() != size {
			return Release{}, fmt.Errorf("zashboard resources are incomplete")
		}
	}
	return meta.Release, nil
}

func safeName(name string) bool {
	if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\:\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimRight(part, " .") != part {
			return false
		}
	}
	return true
}

type progressWriter struct {
	manager *Manager
	ctx     context.Context
}

func (writer progressWriter) Write(data []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		return 0, err
	}
	writer.manager.mu.Lock()
	writer.manager.state.ReceivedBytes += int64(len(data))
	writer.manager.mu.Unlock()
	return len(data), nil
}

func (m *Manager) install(ctx context.Context, release Release) error {
	if err := os.MkdirAll(m.staging, 0700); err != nil {
		return fmt.Errorf("create zashboard staging directory: %w", err)
	}
	stage, err := os.MkdirTemp(m.staging, "download-")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stage)
		}
	}()
	archivePath := filepath.Join(stage, "dist.zip")
	file, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	response, err := m.request(ctx, release.address)
	if err != nil {
		file.Close()
		return err
	}
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(file, hash, progressWriter{manager: m, ctx: ctx}), io.LimitReader(response.Body, release.Size+1))
	response.Body.Close()
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if count != release.Size || hex.EncodeToString(hash.Sum(nil)) != release.SHA256 {
		return fmt.Errorf("zashboard archive size or SHA-256 mismatch")
	}
	m.mu.Lock()
	m.state.Phase = "installing"
	m.mu.Unlock()
	candidate := filepath.Join(stage, "candidate")
	if err := os.MkdirAll(filepath.Join(candidate, "dist"), 0700); err != nil {
		return err
	}
	files, err := extract(ctx, archivePath, filepath.Join(candidate, "dist"))
	if err != nil {
		return err
	}
	data, err := json.Marshal(metadata{Release: release, Files: files})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(candidate, "metadata.json"), data, 0600); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(m.root, 0700); err != nil {
		return err
	}
	destination := filepath.Join(m.root, release.Version)
	backup := filepath.Join(stage, "previous")
	previous := false
	if _, err := os.Lstat(destination); err == nil {
		if err := os.Rename(destination, backup); err != nil {
			return err
		}
		previous = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(candidate, destination); err != nil {
		if previous {
			if restoreErr := os.Rename(backup, destination); restoreErr != nil {
				cleanup = false
				return fmt.Errorf("install zashboard failed; previous cache retained in staging")
			}
		}
		return err
	}
	return nil
}

// Accept both official dist/index.html archives and flat index.html archives.
// No links, duplicate files, paths outside the root, or unbounded expansion.
func extract(ctx context.Context, archivePath, directory string) (map[string]int64, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("invalid zashboard ZIP")
	}
	defer archive.Close()
	if len(archive.File) > maxFiles {
		return nil, fmt.Errorf("too many zashboard files")
	}
	prefix := ""
	for _, file := range archive.File {
		if file.Name == "dist/index.html" {
			prefix = "dist/"
			break
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files := map[string]int64{}
	seen := map[string]bool{}
	var total int64
	for _, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(file.Name, "/")
		if !safeName(name) || file.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unsafe zashboard ZIP entry")
		}
		if prefix != "" && name == "dist" && file.FileInfo().IsDir() {
			continue
		}
		if prefix != "" {
			var ok bool
			name, ok = strings.CutPrefix(name, prefix)
			if !ok {
				return nil, fmt.Errorf("unexpected zashboard ZIP layout")
			}
		}
		if !safeName(name) || seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("duplicate or unsafe zashboard ZIP entry")
		}
		seen[strings.ToLower(name)] = true
		if file.FileInfo().IsDir() {
			if err := root.MkdirAll(name, 0700); err != nil {
				return nil, err
			}
			continue
		}
		if !file.Mode().IsRegular() || file.UncompressedSize64 > uint64(maxExpandedBytes) {
			return nil, fmt.Errorf("unsafe zashboard ZIP entry")
		}
		total += int64(file.UncompressedSize64)
		if total > maxExpandedBytes {
			return nil, fmt.Errorf("zashboard resources exceed size limit")
		}
		if err := root.MkdirAll(path.Dir(name), 0700); err != nil {
			return nil, err
		}
		input, err := file.Open()
		if err != nil {
			return nil, err
		}
		output, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return nil, err
		}
		n, err := io.Copy(output, io.LimitReader(input, int64(file.UncompressedSize64)+1))
		input.Close()
		closeErr := output.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if n != int64(file.UncompressedSize64) {
			return nil, fmt.Errorf("zashboard resource size mismatch")
		}
		files[name] = n
	}
	if files["index.html"] <= 0 {
		return nil, fmt.Errorf("zashboard archive has no index.html")
	}
	return files, nil
}
