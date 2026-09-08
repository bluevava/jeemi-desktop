package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"jeemi/internal/platform/macnetwork"
)

type fileJournal struct{ path string }

func (f fileJournal) Load() (JournalState, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return JournalState{Version: 1, Original: Services{}}, nil
	}
	if err != nil || len(data) > macnetwork.MaxMessageBytes {
		return JournalState{}, macnetwork.Failure("recovery_failed")
	}
	var state JournalState
	if json.Unmarshal(data, &state) != nil || state.Version != 1 || len(state.Original) > 256 {
		return JournalState{}, macnetwork.Failure("recovery_failed")
	}
	return state, nil
}
func (f fileJournal) Save(state JournalState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(data) > macnetwork.MaxMessageBytes {
		return macnetwork.Failure("recovery_failed")
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".journal-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), f.path)
}
func (f fileJournal) Clear() error {
	err := os.Remove(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
