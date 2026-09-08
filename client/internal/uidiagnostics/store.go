// Package uidiagnostics stores bounded, structural renderer diagnostics. It
// deliberately has no fields for raw messages, URLs, stacks or configuration.
package uidiagnostics

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"
)

const (
	maxRecords          = 50
	maxFileBytes        = 128 * 1024
	maxReportsPerMinute = 20
)

type Frame struct {
	Asset  string `json:"asset"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type Failure struct {
	Kind      string  `json:"kind"`
	Scope     string  `json:"scope"`
	Page      string  `json:"page"`
	ErrorType string  `json:"errorType"`
	Code      string  `json:"code"`
	Frames    []Frame `json:"frames"`
}

type Record struct {
	Time       string  `json:"time"`
	AppVersion string  `json:"appVersion"`
	Failure    Failure `json:"failure"`
}

type Store struct {
	mu          sync.Mutex
	directory   string
	version     string
	windowStart time.Time
	windowCount int
	now         func() time.Time
}

// New does no I/O: diagnostic availability must not gate application startup.
func New(dataDirectory, version string) *Store {
	return &Store{directory: filepath.Join(dataDirectory, "diagnostics"), version: version, now: time.Now}
}

var assetName = regexp.MustCompile(`^index-[A-Za-z0-9_-]{8,32}\.js$`)

func validate(input Failure) bool {
	if !slices.Contains([]string{"render", "unhandled_error", "unhandled_rejection", "manual_reload"}, input.Kind) ||
		!slices.Contains([]string{"app", "page", "status", "global"}, input.Scope) ||
		!slices.Contains([]string{"home", "subscriptions", "config", "connections", "logs", "tools", "home_settings", "subscriptions_settings", "config_settings", "connections_settings", "logs_settings", "tools_settings", "script_editor", "config_editor", "unknown"}, input.Page) ||
		!slices.Contains([]string{"Error", "TypeError", "RangeError", "ReferenceError", "SyntaxError", "DOMException", "unknown"}, input.ErrorType) ||
		!slices.Contains([]string{"unknown", "render_loop", "hook_order", "invalid_child", "invalid_value", "stack_overflow", "asset_load"}, input.Code) || len(input.Frames) > 5 {
		return false
	}
	for _, frame := range input.Frames {
		if !assetName.MatchString(frame.Asset) || frame.Line < 1 || frame.Line > 9999999 || frame.Column < 1 || frame.Column > 9999999 {
			return false
		}
	}
	return true
}

func (s *Store) Report(input Failure) error {
	if !validate(input) {
		return errors.New("invalid UI diagnostic")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if now.Sub(s.windowStart) >= time.Minute {
		s.windowStart, s.windowCount = now, 0
	}
	if s.windowCount >= maxReportsPerMinute {
		return errors.New("UI diagnostic rate limit reached")
	}
	s.windowCount++
	if _, err := s.Directory(); err != nil {
		return err
	}
	path := filepath.Join(s.directory, "ui-errors.json")
	records := readRecords(path)
	records = append(records, Record{Time: now.UTC().Format(time.RFC3339Nano), AppVersion: s.version, Failure: input})
	if len(records) > maxRecords {
		records = records[len(records)-maxRecords:]
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil || len(data) > maxFileBytes {
		return errors.New("could not encode UI diagnostic")
	}
	stage, err := os.CreateTemp(s.directory, ".ui-errors-*.tmp")
	if err != nil {
		return errors.New("could not save UI diagnostic")
	}
	defer os.Remove(stage.Name())
	if _, err = stage.Write(data); err == nil {
		err = stage.Sync()
	}
	closeErr := stage.Close()
	if err != nil || closeErr != nil {
		return errors.New("could not save UI diagnostic")
	}
	if err := os.Rename(stage.Name(), path); err != nil {
		return errors.New("could not save UI diagnostic")
	}
	return nil
}

func readRecords(path string) []Record {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil || len(data) > maxFileBytes {
		return nil
	}
	var records []Record
	if json.Unmarshal(data, &records) != nil {
		return nil
	}
	for _, record := range records {
		if !validate(record.Failure) {
			return nil
		}
	}
	return records
}

// Directory exposes a fixed managed directory only to the Go application.
func (s *Store) Directory() (string, error) {
	if err := os.MkdirAll(s.directory, 0o700); err != nil {
		return "", errors.New("UI diagnostics directory is unavailable")
	}
	return s.directory, nil
}
