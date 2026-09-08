package core

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxBinaryBytes = 192 << 20

func (m *VersionManager) downloadAndInstall(ctx context.Context, asset releaseAsset) error {
	if err := os.MkdirAll(m.coreDirectory, 0o700); err != nil {
		return fmt.Errorf("create mihomo core directory: %w", err)
	}
	archive, err := os.CreateTemp(m.coreDirectory, ".download-*")
	if err != nil {
		return fmt.Errorf("create mihomo download: %w", err)
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)
	if err := archive.Chmod(0o600); err != nil {
		archive.Close()
		return fmt.Errorf("protect mihomo download: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		archive.Close()
		return fmt.Errorf("create mihomo download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "Jeemi-mihomo-version-manager")
	response, err := m.httpClient.Do(request)
	if err != nil {
		archive.Close()
		return fmt.Errorf("download mihomo asset: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		archive.Close()
		return fmt.Errorf("download mihomo asset: unexpected HTTP status %d", response.StatusCode)
	}
	if response.ContentLength > maxArchiveBytes || (response.ContentLength >= 0 && asset.Size > 0 && response.ContentLength != asset.Size) {
		archive.Close()
		return fmt.Errorf("download mihomo asset: unexpected archive size")
	}

	hasher := sha256.New()
	written, err := copyWithContext(ctx, io.MultiWriter(archive, hasher), response.Body, maxArchiveBytes)
	if err != nil {
		archive.Close()
		return fmt.Errorf("download mihomo asset: %w", err)
	}
	if written != asset.Size {
		archive.Close()
		return fmt.Errorf("download mihomo asset: archive size mismatch")
	}
	if digest := hex.EncodeToString(hasher.Sum(nil)); digest != asset.SHA256 {
		archive.Close()
		return fmt.Errorf("download mihomo asset: SHA-256 mismatch")
	}
	if err := archive.Sync(); err != nil {
		archive.Close()
		return fmt.Errorf("sync mihomo archive: %w", err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close mihomo archive: %w", err)
	}

	return m.installArchive(ctx, archivePath, asset)
}

func (m *VersionManager) installArchive(ctx context.Context, archivePath string, asset releaseAsset) error {
	versionDirectory := filepath.Join(m.coreDirectory, asset.Version)
	if err := os.MkdirAll(versionDirectory, 0o700); err != nil {
		return fmt.Errorf("create mihomo version directory: %w", err)
	}
	defer removeDirectoryIfEmpty(versionDirectory)
	stageDirectory, err := os.MkdirTemp(versionDirectory, ".install-*")
	if err != nil {
		return fmt.Errorf("create mihomo install stage: %w", err)
	}
	defer os.RemoveAll(stageDirectory)

	executablePath := filepath.Join(stageDirectory, m.target.ExecutableName)
	switch m.target.ArchiveExtension {
	case ".zip":
		err = extractZipExecutable(ctx, archivePath, executablePath)
	case ".gz":
		err = extractGzipExecutable(ctx, archivePath, executablePath)
	default:
		err = fmt.Errorf("unsupported mihomo archive format")
	}
	if err != nil {
		return err
	}
	if err := validateExecutableMagic(executablePath, m.target.OS); err != nil {
		return err
	}
	if err := os.Chmod(executablePath, 0o700); err != nil {
		return fmt.Errorf("mark mihomo executable: %w", err)
	}
	info, err := os.Stat(executablePath)
	if err != nil {
		return fmt.Errorf("inspect mihomo executable: %w", err)
	}
	binaryDigest, err := sha256File(executablePath)
	if err != nil {
		return err
	}
	metadata := installationMetadata{
		Version:       asset.Version,
		Target:        m.target.ID,
		AssetName:     asset.Name,
		ArchiveSHA256: asset.SHA256,
		BinarySHA256:  binaryDigest,
		BinarySize:    info.Size(),
		InstalledAt:   m.now().UTC().Format("2006-01-02T15:04:05Z"),
		Source:        InstallationSourceOfficial,
	}
	metadataContents, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode mihomo metadata: %w", err)
	}
	metadataContents = append(metadataContents, '\n')
	if err := os.WriteFile(filepath.Join(stageDirectory, "metadata.json"), metadataContents, 0o600); err != nil {
		return fmt.Errorf("write mihomo metadata: %w", err)
	}

	targetDirectory := filepath.Join(versionDirectory, m.target.ID)
	if err := promoteDirectory(stageDirectory, targetDirectory, m.coreDirectory); err != nil {
		return fmt.Errorf("install mihomo version: %w", err)
	}
	return nil
}

func (m *VersionManager) importLocal(ctx context.Context, sourcePath string) (string, error) {
	cleanSource, err := filepath.Abs(sourcePath)
	if err != nil || strings.TrimSpace(sourcePath) == "" {
		return "", fmt.Errorf("resolve imported mihomo file")
	}
	info, err := os.Lstat(cleanSource)
	if err != nil {
		return "", fmt.Errorf("inspect imported mihomo file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > maxArchiveBytes {
		return "", fmt.Errorf("imported mihomo file must be a bounded regular file")
	}
	if err := os.MkdirAll(m.coreDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create mihomo core directory: %w", err)
	}

	stageDirectory, err := os.MkdirTemp(m.coreDirectory, ".manual-import-*")
	if err != nil {
		return "", fmt.Errorf("create mihomo import stage: %w", err)
	}
	defer os.RemoveAll(stageDirectory)
	stagedSource := filepath.Join(stageDirectory, "source")
	if err := copyLocalFile(ctx, cleanSource, stagedSource, maxArchiveBytes, 0o600); err != nil {
		return "", fmt.Errorf("stage imported mihomo file: %w", err)
	}
	archiveDigest, err := sha256File(stagedSource)
	if err != nil {
		return "", err
	}

	executablePath := filepath.Join(stageDirectory, m.target.ExecutableName)
	sourceName := filepath.Base(cleanSource)
	if !strings.HasPrefix(strings.ToLower(sourceName), "mihomo") {
		return "", fmt.Errorf("imported core filename must start with mihomo")
	}
	detectedNames := []string{sourceName}
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".zip":
		entryName, extractErr := extractZipExecutableWithName(ctx, stagedSource, executablePath)
		if extractErr != nil {
			return "", extractErr
		}
		detectedNames = append(detectedNames, entryName)
	case ".gz":
		if err := extractGzipExecutable(ctx, stagedSource, executablePath); err != nil {
			return "", err
		}
	default:
		if m.target.OS == "windows" && !strings.EqualFold(filepath.Ext(sourceName), ".exe") {
			return "", fmt.Errorf("Windows mihomo imports must be ZIP or EXE files")
		}
		if err := copyLocalFile(ctx, stagedSource, executablePath, maxBinaryBytes, 0o700); err != nil {
			return "", fmt.Errorf("stage imported mihomo executable: %w", err)
		}
	}
	if err := validateExecutableMagic(executablePath, m.target.OS); err != nil {
		return "", err
	}
	if err := os.Chmod(executablePath, 0o700); err != nil {
		return "", fmt.Errorf("mark imported mihomo executable: %w", err)
	}
	binaryInfo, err := os.Stat(executablePath)
	if err != nil {
		return "", fmt.Errorf("inspect imported mihomo executable: %w", err)
	}
	binaryDigest, err := sha256File(executablePath)
	if err != nil {
		return "", err
	}
	version := detectImportedVersion(detectedNames...)
	if version == "" {
		version = "unknown-" + binaryDigest[:16]
	}

	versionDirectory := filepath.Join(m.coreDirectory, version)
	targetDirectory := filepath.Join(versionDirectory, m.target.ID)
	if err := ensureManagedDescendant(targetDirectory, m.coreDirectory); err != nil {
		return "", err
	}
	if existingDigest, readErr := sha256File(filepath.Join(targetDirectory, m.target.ExecutableName)); readErr == nil {
		if existingDigest == binaryDigest {
			metadataContents, metadataErr := os.ReadFile(filepath.Join(targetDirectory, "metadata.json"))
			var existing installationMetadata
			if metadataErr == nil && json.Unmarshal(metadataContents, &existing) == nil &&
				existing.Version == version && existing.Target == m.target.ID &&
				validInstallationAsset(m.target, existing) && existing.BinarySHA256 == binaryDigest {
				return version, nil
			}
		} else {
			return "", fmt.Errorf("mihomo %s already exists with different contents", version)
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return "", fmt.Errorf("inspect existing mihomo version: %w", readErr)
	}

	metadata := installationMetadata{
		Version: version, Target: m.target.ID, AssetName: sourceName,
		ArchiveSHA256: archiveDigest, BinarySHA256: binaryDigest, BinarySize: binaryInfo.Size(),
		InstalledAt: m.now().UTC().Format("2006-01-02T15:04:05Z"), Source: InstallationSourceManual,
	}
	metadataContents, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode imported mihomo metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stageDirectory, "metadata.json"), append(metadataContents, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write imported mihomo metadata: %w", err)
	}
	if err := os.Remove(stagedSource); err != nil {
		return "", fmt.Errorf("finish mihomo import stage: %w", err)
	}
	if err := os.MkdirAll(versionDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create imported mihomo version directory: %w", err)
	}
	defer removeDirectoryIfEmpty(versionDirectory)
	if err := promoteDirectory(stageDirectory, targetDirectory, m.coreDirectory); err != nil {
		return "", fmt.Errorf("install imported mihomo version: %w", err)
	}
	return version, nil
}

func detectImportedVersion(names ...string) string {
	for _, name := range names {
		matches := versionInNamePattern.FindStringSubmatch(name)
		if len(matches) > 1 && stableVersionPattern.MatchString(matches[1]) {
			return matches[1]
		}
	}
	return ""
}

func extractZipExecutable(ctx context.Context, archivePath, destinationPath string) error {
	_, err := extractZipExecutableWithName(ctx, archivePath, destinationPath)
	return err
}

func extractZipExecutableWithName(ctx context.Context, archivePath, destinationPath string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open mihomo ZIP: %w", err)
	}
	defer reader.Close()

	var candidate *zip.File
	for _, file := range reader.File {
		baseName := filepath.Base(file.Name)
		if !file.FileInfo().Mode().IsRegular() ||
			!strings.EqualFold(filepath.Ext(baseName), ".exe") ||
			!strings.HasPrefix(strings.ToLower(baseName), "mihomo") {
			continue
		}
		if candidate != nil {
			return "", fmt.Errorf("mihomo ZIP contains multiple executables")
		}
		candidate = file
	}
	if candidate == nil || candidate.UncompressedSize64 > maxBinaryBytes {
		return "", fmt.Errorf("mihomo ZIP does not contain one bounded executable")
	}
	source, err := candidate.Open()
	if err != nil {
		return "", fmt.Errorf("open mihomo executable in ZIP: %w", err)
	}
	defer source.Close()
	if err := writeExecutable(ctx, destinationPath, source); err != nil {
		return "", err
	}
	return filepath.Base(candidate.Name), nil
}

func extractGzipExecutable(ctx context.Context, archivePath, destinationPath string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open mihomo gzip: %w", err)
	}
	defer archive.Close()
	reader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open mihomo gzip stream: %w", err)
	}
	defer reader.Close()
	return writeExecutable(ctx, destinationPath, reader)
}

func writeExecutable(ctx context.Context, destinationPath string, source io.Reader) error {
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return fmt.Errorf("create mihomo executable: %w", err)
	}
	written, copyErr := copyWithContext(ctx, destination, source, maxBinaryBytes)
	syncErr := destination.Sync()
	closeErr := destination.Close()
	if copyErr != nil {
		return fmt.Errorf("extract mihomo executable: %w", copyErr)
	}
	if written == 0 {
		return fmt.Errorf("extract mihomo executable: empty output")
	}
	if syncErr != nil {
		return fmt.Errorf("sync mihomo executable: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close mihomo executable: %w", closeErr)
	}
	return nil
}

func copyLocalFile(ctx context.Context, sourcePath, destinationPath string, limit int64, permissions os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, permissions)
	if err != nil {
		return err
	}
	_, copyErr := copyWithContext(ctx, destination, source, limit)
	syncErr := destination.Sync()
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader, limit int64) (int64, error) {
	buffer := make([]byte, 128<<10)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		count, readErr := source.Read(buffer)
		if count > 0 {
			written += int64(count)
			if written > limit {
				return written, fmt.Errorf("content exceeds size limit")
			}
			if _, err := destination.Write(buffer[:count]); err != nil {
				return written, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

func validateExecutableMagic(path, goos string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open mihomo executable for validation: %w", err)
	}
	defer file.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("read mihomo executable header: %w", err)
	}
	valid := false
	switch goos {
	case "windows":
		valid = header[0] == 'M' && header[1] == 'Z'
	case "linux":
		valid = string(header) == "\x7fELF"
	case "darwin":
		magic := hex.EncodeToString(header)
		valid = magic == "cffaedfe" || magic == "feedfacf" || magic == "cafebabe" || magic == "bebafeca"
	}
	if !valid {
		return fmt.Errorf("mihomo archive does not contain a valid %s executable", goos)
	}
	return nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for SHA-256: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("calculate file SHA-256: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func promoteDirectory(stagePath, targetPath, managedRoot string) error {
	if err := ensureManagedDescendant(stagePath, managedRoot); err != nil {
		return err
	}
	if err := ensureManagedDescendant(targetPath, managedRoot); err != nil {
		return err
	}
	backupPath := targetPath + ".previous"
	if err := ensureManagedDescendant(backupPath, managedRoot); err != nil {
		return err
	}
	if err := os.RemoveAll(backupPath); err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stagePath, targetPath); err != nil {
		if _, backupErr := os.Stat(backupPath); backupErr == nil {
			_ = os.Rename(backupPath, targetPath)
		}
		return err
	}
	return os.RemoveAll(backupPath)
}

func removeDirectoryIfEmpty(path string) {
	entries, err := os.ReadDir(path)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(path)
	}
}
