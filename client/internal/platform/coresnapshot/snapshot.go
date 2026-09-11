package coresnapshot

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"jeemi/internal/externalui"

	"gopkg.in/yaml.v3"
	"jeemi/internal/platform/requirements"
	"path/filepath"
	"runtime"
)

const MaxConfigurationBytes = 1 << 20

func failure(code string) error {
	return &requirements.Error{Code: "authorization_snapshot_" + code, Message: "authorization_snapshot_" + code}
}

const snapshotLimit int64 = 1 << 30
const snapshotFileLimit int64 = 256 << 20

func ConfigurationName(name string) bool {
	parts := strings.Split(name, "/")
	return len(parts) == 3 && parts[0] == "generations" && parts[1] != "" && parts[1] != "." && parts[1] != ".." && len(parts[1]) < 160 && (parts[2] == "bootstrap.yaml" || parts[2] == "config.yaml") && path.Clean(name) == name && !strings.ContainsAny(name, "\\:\x00\r\n")
}

// WriteSnapshot runs only as the ordinary GUI user (or a child that has
// dropped all root credentials). It never reads user input as root.
func WriteSnapshot(root, configuration string, initial bool, output io.Writer) error {
	if !ConfigurationName(configuration) {
		return failure("unsafe_path")
	}
	source, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer source.Close()
	uiDirectory := ""
	if initial {
		file, err := source.Open(configuration)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(file, MaxConfigurationBytes+1))
		file.Close()
		if err != nil || len(data) > MaxConfigurationBytes {
			return failure("invalid_request")
		}
		var fields struct {
			ExternalUI string `yaml:"external-ui"`
		}
		if yaml.Unmarshal(data, &fields) != nil {
			return failure("invalid_request")
		}
		parts := strings.Split(fields.ExternalUI, "/")
		if len(parts) == 4 && parts[0] == "external-ui" && parts[1] == "zashboard" && externalui.ValidVersion(parts[2]) && parts[3] == "dist" {
			uiDirectory = fields.ExternalUI
		}
	}
	archive := tar.NewWriter(output)
	defer archive.Close()
	total := int64(0)
	count := 0
	err = fs.WalkDir(source.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		// The persistent UI cache may contain several versions. Send only the
		// active release at process start, and no UI files during hot reloads.
		if name == "external-ui" || strings.HasPrefix(name, "external-ui/") {
			include := uiDirectory != "" && (name == uiDirectory || strings.HasPrefix(name, uiDirectory+"/") || (entry.IsDir() && strings.HasPrefix(uiDirectory, name+"/")))
			if !include {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		if entry.IsDir() {
			if name == "resolved" || (strings.HasPrefix(name, "generations/") && name != path.Dir(configuration)) {
				return fs.SkipDir
			}
			if !initial && name != "generations" && name != path.Dir(configuration) && name != "rule-providers" && !strings.HasPrefix(name, "rule-providers/") {
				return fs.SkipDir
			}
			return nil
		}
		if !initial && !strings.HasPrefix(name, path.Dir(configuration)+"/") && !strings.HasPrefix(name, "rule-providers/") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return failure("unsafe_path")
		}
		if info.Size() > snapshotFileLimit {
			return failure("unsafe_path")
		}
		total += info.Size()
		count++
		if total > snapshotLimit || count > 10000 {
			return failure("unsafe_path")
		}
		file, err := source.Open(name)
		if err != nil {
			return err
		}
		defer file.Close()
		if err = archive.WriteHeader(&tar.Header{Name: name, Size: info.Size(), Mode: 0600, Typeflag: tar.TypeReg, ModTime: info.ModTime()}); err != nil {
			return err
		}
		_, err = io.CopyN(archive, file, info.Size())
		return err
	})
	if err != nil {
		return err
	}
	return archive.Close()
}

// receiveSnapshot creates independent bytes in root-owned storage, never
// links to user files. Repeated input does not overwrite live provider caches.
func Receive(directory, userRuntime string, input io.Reader, previous map[string]string) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	reader := tar.NewReader(io.LimitReader(input, snapshotLimit+(16<<20)))
	seen := map[string]bool{}
	total := int64(0)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name := header.Name
		if name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00\r\n") || seen[name] || header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > snapshotFileLimit {
			return failure("unsafe_path")
		}
		total += header.Size
		seen[name] = true
		if total > snapshotLimit || len(seen) > 10000 {
			return failure("unsafe_path")
		}
		if err = root.MkdirAll(path.Dir(name), 0700); err != nil {
			return err
		}
		temp := name + ".incoming"
		file, err := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		hash := sha256.New()
		if ConfigurationName(name) {
			if header.Size > MaxConfigurationBytes {
				file.Close()
				root.Remove(temp)
				return failure("invalid_request")
			}
			data, readErr := io.ReadAll(reader)
			if readErr != nil {
				err = readErr
			} else {
				hash.Write(data)
				data, err = rewriteRuntimePaths(data, userRuntime, directory)
				if err == nil && strings.HasSuffix(name, "/bootstrap.yaml") {
					err = checkBootstrap(data)
				}
				if err == nil {
					_, err = file.Write(data)
				}
			}
		} else {
			_, err = io.Copy(io.MultiWriter(file, hash), reader)
		}
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			root.Remove(temp)
			return err
		}
		digest := hex.EncodeToString(hash.Sum(nil))
		if previous[name] == digest {
			root.Remove(temp)
			continue
		}
		if !ConfigurationName(name) && !header.ModTime.IsZero() {
			if err = root.Chtimes(temp, header.ModTime, header.ModTime); err != nil {
				root.Remove(temp)
				return err
			}
		}
		if err = root.Rename(temp, name); err != nil {
			root.Remove(temp)
			return err
		}
		previous[name] = digest
	}
	return nil
}
func rewriteRuntimePaths(data []byte, source, target string) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, failure("invalid_request")
	}
	var walk func(*yaml.Node, int) error
	count := 0
	walk = func(node *yaml.Node, depth int) error {
		count++
		if depth > 64 || count > 50000 {
			return failure("invalid_request")
		}
		if node.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i+1 < len(node.Content); i += 2 {
				key := node.Content[i]
				if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
					return failure("invalid_request")
				}
				seen[key.Value] = true
			}
		}
		if node.Kind == yaml.AliasNode {
			return failure("invalid_request")
		}
		if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
			value := filepath.ToSlash(node.Value)
			if strings.HasPrefix(value, filepath.ToSlash(source)+"/") {
				node.Value = filepath.ToSlash(target) + strings.TrimPrefix(value, filepath.ToSlash(source))
			}
			if runtime.GOOS == "windows" {
				if err := safeWindowsPath(node.Value, target); err != nil {
					return err
				}
			}
		}
		for _, child := range node.Content {
			if err := walk(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, failure("invalid_request")
	}
	root := document.Content[0]
	if runtime.GOOS == "windows" {
		if ntp := mappingValue(root, "ntp"); ntp != nil {
			if field := mappingValue(ntp, "write-to-system"); field != nil && field.Value != "false" {
				return nil, failure("invalid_request")
			}
		}
	}
	controller := scalarValue(root, "external-controller")
	host, port, err := net.SplitHostPort(controller)
	number, parseErr := strconv.Atoi(port)
	if err != nil || parseErr != nil || host != "127.0.0.1" || number < 1 || number > 65535 || len(scalarValue(root, "secret")) < 32 || scalarValue(root, "external-controller-tls") != "" || scalarValue(root, "external-controller-unix") != "" || scalarValue(root, "external-controller-pipe") != "" {
		return nil, failure("invalid_request")
	}
	if err := walk(&document, 0); err != nil {
		return nil, err
	}
	result, err := yaml.Marshal(&document)
	if err != nil {
		return nil, fmt.Errorf("encode protected configuration")
	}
	return result, nil
}

// A privileged Windows core has no sandbox-exec. All file-looking scalars
// must remain in the independent session tree, including drive/UNC paths and
// traversal. URLs and inline PEM values are data rather than file paths.
func safeWindowsPath(value, root string) error {
	if strings.Contains(value, "\x00") {
		return failure("unsafe_path")
	}
	if strings.Contains(value, "://") || strings.HasPrefix(value, "-----BEGIN ") {
		return nil
	}
	normalized := strings.ReplaceAll(value, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return failure("unsafe_path")
		}
	}
	absolute := strings.HasPrefix(normalized, "/") || (len(normalized) > 1 && normalized[1] == ':')
	if absolute && !strings.HasPrefix(strings.ToLower(normalized), strings.ToLower(strings.ReplaceAll(root, "\\", "/"))+"/") {
		return failure("unsafe_path")
	}
	return nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
func scalarValue(node *yaml.Node, key string) string {
	value := mappingValue(node, key)
	if value != nil && value.Kind == yaml.ScalarNode && value.Tag == "!!str" {
		return value.Value
	}
	return ""
}
func checkBootstrap(data []byte) error {
	var document yaml.Node
	if yaml.Unmarshal(data, &document) != nil || len(document.Content) != 1 {
		return failure("invalid_request")
	}
	tun := mappingValue(document.Content[0], "tun")
	if tun != nil {
		enabled := mappingValue(tun, "enable")
		if enabled == nil || enabled.Tag != "!!bool" || enabled.Value != "false" {
			return failure("invalid_request")
		}
	}
	return nil
}
