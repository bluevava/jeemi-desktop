package appupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// All archive entries are bounded before any install directory is touched.
// macOS bundle links are created last and must resolve inside the bundle.
func extract(ctx context.Context, archive, output string, t target) error {
	x := extractor{ctx: ctx, root: output, target: t, names: map[string]bool{}}
	if t.extension == ".zip" {
		reader, err := zip.OpenReader(archive)
		if err != nil {
			return ErrPackage
		}
		defer reader.Close()
		if len(reader.File) > 10000 {
			return ErrPackage
		}
		for _, file := range reader.File {
			if file.UncompressedSize64 > uint64(maxExtractBytes) {
				return ErrPackage
			}
			body, err := file.Open()
			if err != nil {
				return ErrPackage
			}
			err = x.entry(file.Name, file.Mode(), int64(file.UncompressedSize64), body)
			body.Close()
			if err != nil {
				return err
			}
		}
	} else {
		file, err := os.Open(archive)
		if err != nil {
			return ErrPackage
		}
		defer file.Close()
		gz, err := gzip.NewReader(file)
		if err != nil {
			return ErrPackage
		}
		defer gz.Close()
		reader := tar.NewReader(gz)
		for count := 0; ; count++ {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil || count >= 10000 {
				return ErrPackage
			}
			if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
				return ErrPackage
			}
			if err = x.entry(header.Name, header.FileInfo().Mode(), header.Size, reader); err != nil {
				return err
			}
		}
	}
	for _, link := range x.links {
		if err := os.Symlink(link.target, link.name); err != nil {
			return ErrPackage
		}
	}
	for _, link := range x.links {
		resolved, err := filepath.EvalSymlinks(link.name)
		if err != nil || !within(filepath.Join(output, t.directory, "Jeemi.app"), resolved) {
			return ErrPackage
		}
	}
	return nil
}

type extractor struct {
	ctx    context.Context
	root   string
	target target
	size   int64
	names  map[string]bool
	links  []struct{ name, target string }
}

func safeArchivePath(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		// Reject Windows device names even when inspecting a package elsewhere.
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}

func within(root, filename string) bool {
	rel, err := filepath.Rel(root, filename)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func (x *extractor) entry(name string, mode os.FileMode, size int64, reader io.Reader) error {
	if x.ctx.Err() != nil {
		return ErrCancelled
	}
	if !safeArchivePath(name) || size < 0 || size > maxExtractBytes-x.size {
		return ErrPackage
	}
	x.size += size
	// ditto's AppleDouble metadata is not application code or signed resources.
	if x.target.platform == "macos" && (name == "__MACOSX/" || strings.HasPrefix(name, "__MACOSX/")) {
		return nil
	}
	clean := strings.TrimSuffix(name, "/")
	if clean != x.target.directory && !strings.HasPrefix(clean, x.target.directory+"/") {
		return ErrPackage
	}
	key := strings.ToLower(clean)
	if x.names[key] {
		return ErrPackage
	}
	x.names[key] = true
	filename := filepath.Join(x.root, filepath.FromSlash(clean))
	if !within(x.root, filename) {
		return ErrPackage
	}
	if mode.IsDir() {
		if err := os.MkdirAll(filename, 0755); err != nil {
			return ErrPackage
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return ErrPackage
	}
	if mode&os.ModeSymlink != 0 {
		if x.target.platform != "macos" || size > 4096 || !strings.HasPrefix(clean, x.target.directory+"/Jeemi.app/") {
			return ErrPackage
		}
		body, err := io.ReadAll(io.LimitReader(reader, 4097))
		if err != nil || len(body) > 4096 {
			return ErrPackage
		}
		link := string(body)
		if link == "" || path.IsAbs(link) || strings.ContainsAny(link, "\\:\x00") {
			return ErrPackage
		}
		resolved := path.Clean(path.Join(path.Dir(clean), link))
		if !strings.HasPrefix(resolved, x.target.directory+"/Jeemi.app/") {
			return ErrPackage
		}
		x.links = append(x.links, struct{ name, target string }{filename, link})
		return nil
	}
	if !mode.IsRegular() {
		return ErrPackage
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()&0755|0600)
	if err != nil {
		return ErrPackage
	}
	n, err := io.Copy(file, io.LimitReader(reader, size+1))
	closeErr := file.Close()
	if err != nil || closeErr != nil || n != size {
		return ErrPackage
	}
	return nil
}

type packageManifest struct {
	Product      string `json:"product"`
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Files        []struct {
		Path   string `json:"path"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

func verifyPackage(ctx context.Context, directory, version string, t target) error {
	contents, err := readBounded(filepath.Join(directory, "release.json"), maxReleaseBytes)
	if err != nil {
		return ErrPackage
	}
	var manifest packageManifest
	if json.Unmarshal(contents, &manifest) != nil || manifest.Product != "Jeemi" || manifest.Version != strings.TrimPrefix(version, "v") || manifest.Platform != t.platform || manifest.Architecture != t.architecture || len(manifest.Files) == 0 || len(manifest.Files) > 10000 {
		return ErrPackage
	}
	listed := map[string]bool{}
	var total int64
	for _, file := range manifest.Files {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		if !safeArchivePath(file.Path) || listed[file.Path] || file.Size < 0 || file.Size > maxExtractBytes-total || !digestPattern.MatchString("sha256:"+file.SHA256) {
			return ErrPackage
		}
		total += file.Size
		listed[file.Path] = true
		filename, err := filepath.EvalSymlinks(filepath.Join(directory, filepath.FromSlash(file.Path)))
		if err != nil || !within(directory, filename) {
			return ErrPackage
		}
		digest, size, err := hashFile(filename)
		if err != nil || size != file.Size || digest != file.SHA256 {
			return ErrPackage
		}
	}
	if !listed[t.executable] || !listed[t.helper] {
		return ErrPackage
	}
	// No unsigned/unlisted executable files may ride along in the archive.
	err = filepath.WalkDir(directory, func(filename string, entry os.DirEntry, err error) error {
		if err != nil {
			return ErrPackage
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(directory, filename)
		if err != nil {
			return ErrPackage
		}
		name := filepath.ToSlash(rel)
		if name != "release.json" && name != "SHA256SUMS" && !listed[name] {
			return ErrPackage
		}
		return nil
	})
	if err != nil {
		return ErrPackage
	}
	for _, name := range t.installItems() {
		if _, err := os.Lstat(filepath.Join(directory, name)); err != nil {
			return ErrPackage
		}
	}
	return nil
}

func readBounded(filename string, limit int64) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if int64(len(data)) > limit {
		return nil, ErrPackage
	}
	return data, err
}

func hashFile(filename string) (string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxExtractBytes {
		return "", 0, ErrPackage
	}
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), n, err
}
