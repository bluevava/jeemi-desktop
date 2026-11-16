package subscription

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"jeemi/internal/subscriptionformat"
)

// NormalizeFileSourcePath accepts a path obtained by the native file picker.
// It does not read the source; ordinary metadata reads never touch external files.
func NormalizeFileSourcePath(path string) (string, error) {
	if path == "" || len(path) > 32768 || strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("subscription_file_source_missing")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("subscription_file_unavailable")
	}
	if _, err := formatForImportedFile(absolute); err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func validStoredFileSource(path string) bool {
	// Missing is allowed for existing imports that only recorded a basename.
	return path == "" || len(path) <= 32768 && !strings.ContainsRune(path, '\x00') && filepath.IsAbs(path) && filepath.Clean(path) == path
}

func readFileSource(ctx context.Context, path string) (FetchedContent, error) {
	if err := ctx.Err(); err != nil {
		return FetchedContent{}, err
	}
	if path == "" {
		return FetchedContent{}, fmt.Errorf("subscription_file_source_missing")
	}
	format, err := formatForImportedFile(path)
	if err != nil {
		return FetchedContent{}, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxSubscriptionBytes {
		return FetchedContent{}, fmt.Errorf("subscription_file_unavailable")
	}
	file, err := os.Open(path)
	if err != nil {
		return FetchedContent{}, fmt.Errorf("subscription_file_unavailable")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return FetchedContent{}, fmt.Errorf("subscription_file_changed")
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxSubscriptionBytes+1))
	if err != nil || len(contents) == 0 || len(contents) > maxSubscriptionBytes {
		return FetchedContent{}, fmt.Errorf("subscription_file_unavailable")
	}
	after, err := file.Stat()
	current, currentErr := os.Lstat(path)
	if err != nil || currentErr != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) ||
		after.Size() != int64(len(contents)) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return FetchedContent{}, fmt.Errorf("subscription_file_changed")
	}
	if err := ctx.Err(); err != nil {
		return FetchedContent{}, err
	}
	normalized, err := subscriptionformat.Normalize(contents)
	if err != nil {
		return FetchedContent{}, err
	}
	if normalized.Report.Format != "mihomo" {
		format = FormatText
	}
	if err := validateContents(format, contents); err != nil {
		return FetchedContent{}, err
	}
	return FetchedContent{Format: format, Contents: contents}, nil
}
