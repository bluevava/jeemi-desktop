// Package configworkspace publishes isolated configuration schema generations.
// Call Prepare only after acquiring Jeemi's cross-version single-instance lock.
package configworkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"jeemi/internal/configdocument"
	"jeemi/internal/configfile"
)

const CurrentVersion = 1

// These are configuration-owned trees, including immutable content referenced
// by their manifests. No executable, network journal or live session is copied.
var configurationEntries = []string{
	"settings.json", "subscriptions", "local-configs", "local-scripts",
	"chain-proxies", "geodata", "state/mihomo/proxy-selections",
}

type Options struct {
	Version int
	// Zero identifies the legacy data root. Future schemas explicitly select
	// their predecessor; an app never follows a global "latest" pointer.
	SourceVersion int
	Validate      func(context.Context, string) error
	Migrate       func(context.Context, string) error
	// Fault injection is private; production uses ordinary filesystem calls.
	beforePublish func(string) error
	copyFile      func(context.Context, string, string) (fileRecord, error)
}

type manifest struct {
	Version       int    `json:"version"`
	SchemaVersion int    `json:"schemaVersion"`
	SourceVersion int    `json:"sourceVersion"`
	CreatedAt     string `json:"createdAt"`
}

var prepareMu sync.Mutex

// Check is a read-only readiness check. Missing or future workspaces are never
// interpreted as an empty configuration.
func Check(dataRoot string, version int) error {
	directory, err := Directory(dataRoot, version)
	if err != nil {
		return err
	}
	if err := safePath(dataRoot, directory); err != nil {
		return configfile.Unreadable(directory, err)
	}
	return readManifest(directory, version)
}

func Directory(dataRoot string, version int) (string, error) {
	if !filepath.IsAbs(dataRoot) || version < 1 {
		return "", fmt.Errorf("invalid configuration workspace location")
	}
	return filepath.Join(dataRoot, "config-workspaces", fmt.Sprintf("schema-%d", version)), nil
}

// Prepare copies once, validates the complete candidate, then atomically renames
// its directory. An existing workspace is never overwritten by legacy changes.
func Prepare(ctx context.Context, dataRoot string, options Options) (string, error) {
	prepareMu.Lock()
	defer prepareMu.Unlock()
	if options.Version == 0 {
		options.Version = CurrentVersion
	}
	target, err := Directory(dataRoot, options.Version)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if options.SourceVersion < 0 || options.SourceVersion >= options.Version || options.Validate == nil {
		return "", fmt.Errorf("invalid configuration migration plan")
	}
	if err := safePath(dataRoot, target); err != nil {
		return "", configfile.Unreadable(target, err)
	}
	if _, err := os.Lstat(target); err == nil {
		if err := readManifest(target, options.Version); err != nil {
			return "", err
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", configfile.Unreadable(target, err)
	}
	if options.Version > 1 && options.Migrate == nil {
		return "", fmt.Errorf("configuration schema migration is not registered")
	}
	source := filepath.Clean(dataRoot)
	if options.SourceVersion != 0 {
		source, _ = Directory(dataRoot, options.SourceVersion)
		if err := safePath(dataRoot, source); err != nil {
			return "", configfile.Unreadable(source, err)
		}
		if err := readManifest(source, options.SourceVersion); err != nil {
			return "", err
		}
		if options.Migrate == nil {
			return "", fmt.Errorf("configuration schema migration is not registered")
		}
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", configfile.MigrationFailed(source, err)
	}
	stage, err := os.MkdirTemp(parent, fmt.Sprintf(".schema-%d-stage-", options.Version))
	if err != nil {
		return "", configfile.MigrationFailed(source, err)
	}
	// Only this invocation's private staging directory is removed. Crash leftovers
	// are never selected as a workspace or used to overwrite a committed directory.
	defer os.RemoveAll(stage)
	fail := func(err error) (string, error) { return "", configfile.MigrationFailed(source, err) }
	copyFile := options.copyFile
	if copyFile == nil {
		copyFile = copyRegularFile
	}
	records, err := copyConfiguration(ctx, source, stage, copyFile)
	if err != nil {
		return fail(err)
	}
	if options.Migrate != nil {
		if err := options.Migrate(ctx, stage); err != nil {
			return fail(err)
		}
	}
	if err := options.Validate(ctx, stage); err != nil {
		return fail(err)
	}
	if err := verifySource(ctx, source, records); err != nil {
		return fail(err)
	}
	if err := writeManifest(stage, options.Version, options.SourceVersion); err != nil {
		return fail(err)
	}
	if options.beforePublish != nil {
		if err := options.beforePublish(stage); err != nil {
			return fail(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := safePath(dataRoot, target); err != nil {
		return fail(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return fail(fmt.Errorf("configuration workspace appeared during migration"))
	}
	if err := syncWorkspace(ctx, stage); err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := os.Rename(stage, target); err != nil {
		return fail(err)
	}
	// Publication is the commit point. A subsequent directory-flush failure must
	// never delete the complete new workspace or trigger a second migration.
	if err := syncDirectory(parent); err != nil {
		return "", configfile.MigrationFailed(target, err)
	}
	return target, nil
}

func readManifest(directory string, version int) error {
	path := filepath.Join(directory, "workspace.json")
	if err := safePath(directory, path); err != nil {
		return configfile.Unreadable(path, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return configfile.MigrationFailed(directory, fmt.Errorf("configuration workspace is incomplete: %w", err))
	}
	if len(raw) > 64<<10 {
		return configfile.MigrationFailed(directory, fmt.Errorf("configuration workspace manifest exceeds limit"))
	}
	var value manifest
	if err := (configdocument.Format{VersionField: "version", Version: 1}).Decode(raw, &value); err != nil {
		return configfile.Invalid(path, raw, err)
	}
	if value.SchemaVersion != version {
		return configfile.Invalid(path, raw, configfile.UnsupportedFormat("configuration workspace schema is unsupported"))
	}
	if _, err := time.Parse(time.RFC3339Nano, value.CreatedAt); err != nil || value.SourceVersion < 0 || value.SourceVersion >= version {
		return configfile.MigrationFailed(directory, fmt.Errorf("configuration workspace manifest is inconsistent"))
	}
	return nil
}

func writeManifest(directory string, version, sourceVersion int) error {
	raw, err := json.MarshalIndent(manifest{Version: 1, SchemaVersion: version, SourceVersion: sourceVersion, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(directory, "workspace.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}
