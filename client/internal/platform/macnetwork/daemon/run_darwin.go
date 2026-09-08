//go:build darwin

package daemon

import (
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"jeemi/internal/platform/macnetwork"
)

const rootDirectory = macnetwork.StateDirectory

// The fixed local installer invokes this only after launchd has removed the
// job. No GUI paths/configuration are accepted and a failed journal survives.
func RecoverForRemoval() error {
	if os.Geteuid() != 0 || !macnetwork.LocalTesting {
		return macnetwork.Failure("identity_failed")
	}
	if _, err := os.Lstat(rootDirectory); os.IsNotExist(err) {
		return nil
	}
	if err := rootOwned(rootDirectory, true); err != nil {
		return err
	}
	return NewJournal(fileJournal{path: filepath.Join(rootDirectory, "network-recovery.json")}, nativeNetwork{}).Restore()
}

type nativeNetwork struct{}

func (nativeNetwork) Read() (Services, error) {
	data, err := macnetwork.ReadPreferences()
	if err != nil {
		return nil, err
	}
	var values Services
	if len(data) > macnetwork.MaxMessageBytes || json.Unmarshal(data, &values) != nil {
		return nil, macnetwork.Failure("network_failed")
	}
	return values, nil
}
func (nativeNetwork) Write(values Services) error {
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return macnetwork.WritePreferences(data)
}

func Run() error {
	if os.Geteuid() != 0 {
		return macnetwork.Failure("identity_failed")
	}
	if err := os.MkdirAll(rootDirectory, 0o700); err != nil {
		return err
	}
	if err := rootOwned(rootDirectory, true); err != nil {
		return err
	}
	cores, err := newProtectedCores(rootDirectory)
	if err != nil {
		return err
	}
	journal := NewJournal(fileJournal{path: filepath.Join(rootDirectory, "network-recovery.json")}, nativeNetwork{})
	server := NewServer(cores, journal, openDNS)
	// launchd has reaped the previous process group before restarting this job.
	if entries, err := os.ReadDir(rootDirectory); err == nil {
		for _, entry := range entries {
			removeWorkspace(filepath.Join(rootDirectory, entry.Name()))
		}
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() { <-signals; server.Close(); os.Exit(0) }()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			server.Tick()
		}
	}()
	return macnetwork.Serve(server.Handle, server.Connected, server.Disconnected)
}
