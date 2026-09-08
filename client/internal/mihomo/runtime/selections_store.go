package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const selectionFileLimit = 16 << 20

var selectionSubscriptionID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Access is serialized by Manager.operationMu. Choices intentionally survive
// subscription revisions, core replacement and disposable helper sessions.
type selectionStore struct{ root string }

type selectionDocument struct {
	Version        int               `json:"version"`
	SubscriptionID string            `json:"subscriptionId"`
	Selections     map[string]string `json:"selections"`
}

func selectionNameValid(name string) bool {
	return strings.TrimSpace(name) != "" && len(name) <= 1024 && utf8.ValidString(name) && !strings.ContainsRune(name, 0)
}

func (s selectionStore) path(id string) (string, error) {
	if !selectionSubscriptionID.MatchString(id) {
		return "", fmt.Errorf("invalid proxy selection subscription")
	}
	return filepath.Join(s.root, id+".json"), nil
}

func (s selectionStore) load(id string) (map[string]string, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > selectionFileLimit {
		return nil, fmt.Errorf("proxy selection state is unavailable")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read proxy selection state")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, selectionFileLimit+1))
	if err != nil || len(data) > selectionFileLimit {
		return nil, fmt.Errorf("cannot read proxy selection state")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document selectionDocument
	if decoder.Decode(&document) != nil || decoder.Decode(new(any)) != io.EOF || document.Version != 1 || document.SubscriptionID != id || !validSelections(document.Selections) {
		return nil, fmt.Errorf("invalid proxy selection state")
	}
	return document.Selections, nil
}

func validSelections(values map[string]string) bool {
	if values == nil || len(values) > 4096 {
		return false
	}
	for group, proxy := range values {
		if !selectionNameValid(group) || !selectionNameValid(proxy) {
			return false
		}
	}
	return true
}

func (s selectionStore) merge(id string, values map[string]string) error {
	if !validSelections(values) {
		return fmt.Errorf("invalid proxy selections")
	}
	if len(values) == 0 {
		return nil
	}
	previous, err := s.load(id)
	if err != nil {
		return err
	}
	next := maps.Clone(previous)
	maps.Copy(next, values)
	if maps.Equal(previous, next) {
		return nil
	}
	if !validSelections(next) {
		return fmt.Errorf("too many proxy selections")
	}
	data, err := json.Marshal(selectionDocument{Version: 1, SubscriptionID: id, Selections: next})
	if err != nil || len(data) > selectionFileLimit {
		return fmt.Errorf("cannot encode proxy selection state")
	}
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := replacePrivateFile(path, append(data, '\n')); err != nil {
		return fmt.Errorf("cannot save proxy selection state")
	}
	return nil
}

func (s selectionStore) delete(id string) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot delete proxy selection state")
	}
	return nil
}
