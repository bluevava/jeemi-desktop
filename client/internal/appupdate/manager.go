package appupdate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type State struct {
	Phase      string `json:"phase"`
	Version    string `json:"version"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Error      string `json:"error"`
	JobID      string `json:"jobId"`
}

type Manager struct {
	mu            sync.Mutex
	root, current string
	state         State
	checked       Result
	checkedAt     time.Time
	cancel        context.CancelFunc
	worker        *workerHandle
}

func NewManager(dataRoot, current string) *Manager {
	return &Manager{root: filepath.Join(dataRoot, "updates", "jeemi"), current: current, state: State{Phase: "idle"}}
}

func busy(phase string) bool {
	return phase == "checking" || phase == "downloading" || phase == "verifying" || phase == "restarting"
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Phase == "idle" || (m.state.Phase == "restarting" && m.worker == nil) {
		var receipt State
		if data, err := readBounded(filepath.Join(m.root, "result.json"), 4096); err == nil && json.Unmarshal(data, &receipt) == nil && idPattern.MatchString(receipt.JobID) && (receipt.Phase == "succeeded" || receipt.Phase == "failed") {
			if m.state.JobID == "" || receipt.JobID == m.state.JobID {
				m.state = receipt
			}
		}
	}
	return m.state
}

func (m *Manager) Check(ctx context.Context) (Result, error) {
	m.mu.Lock()
	if busy(m.state.Phase) {
		m.mu.Unlock()
		return Result{}, ErrBusy
	}
	m.state = State{Phase: "checking"}
	m.checked = Result{}
	m.mu.Unlock()
	m.cleanupPreviousJob()
	result, err := Check(ctx, m.current)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.state = State{Phase: "failed", Error: err.Error()}
		return Result{}, err
	}
	m.checked = result
	m.checkedAt = time.Now()
	m.state = State{Phase: "idle", Version: result.LatestVersion}
	return result, nil
}

// Prepare uses only the last checked release. The renderer cannot supply a
// download URL, digest, process command or installation path.
func (m *Manager) Prepare(ctx context.Context, version string) (err error) {
	m.mu.Lock()
	if busy(m.state.Phase) {
		m.mu.Unlock()
		return ErrBusy
	}
	if !m.checked.UpdateAvailable || version != m.checked.LatestVersion || time.Since(m.checkedAt) > 30*time.Minute {
		m.mu.Unlock()
		return ErrExpired
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	m.cancel = cancel
	published := m.checked.release
	m.state = State{Phase: "downloading", Version: version}
	m.mu.Unlock()
	defer func() {
		cancelled := errors.Is(ctx.Err(), context.Canceled)
		cancel()
		m.mu.Lock()
		defer m.mu.Unlock()
		m.cancel = nil
		if err != nil {
			if cancelled {
				err = ErrCancelled
			}
			m.state.Phase = "failed"
			m.state.Error = err.Error()
			if err == ErrCancelled {
				m.state.Phase = "cancelled"
			}
		}
	}()
	t, err := targetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	item, err := published.archive(t)
	if err != nil {
		return err
	}
	current, valid := parseVersion(m.current)
	if !valid || current.prerelease {
		return ErrLocation
	}
	executable, err := os.Executable()
	if err != nil {
		return ErrLocation
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return ErrLocation
	}
	if _, err = t.installRoot(executable); err != nil {
		return err
	}
	idBytes := make([]byte, 16)
	if _, err = rand.Read(idBytes); err != nil {
		return ErrDownload
	}
	id := hex.EncodeToString(idBytes)
	jobRoot := filepath.Join(m.root, id)
	if err = os.MkdirAll(jobRoot, 0700); err != nil {
		return ErrDownload
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(jobRoot)
		}
	}()
	m.mu.Lock()
	m.state.JobID = id
	m.state.Total = item.Size
	m.mu.Unlock()
	archive := filepath.Join(jobRoot, "archive"+t.extension)
	if err = download(ctx, downloadClient(), item, archive, func(n int64) { m.mu.Lock(); m.state.Downloaded = n; m.mu.Unlock() }); err != nil {
		return err
	}
	m.mu.Lock()
	m.state.Phase = "verifying"
	m.mu.Unlock()
	extracted := filepath.Join(jobRoot, "extracted")
	if err = os.Mkdir(extracted, 0700); err != nil {
		return ErrPackage
	}
	if err = extract(ctx, archive, extracted, t); err != nil {
		return err
	}
	if err = verifyPackage(ctx, filepath.Join(extracted, t.directory), version, t); err != nil {
		return err
	}
	digest, _, err := hashFile(executable)
	if err != nil {
		return ErrLocation
	}
	job := installJob{ID: id, Executable: executable, CurrentSHA256: digest, Version: version}
	worker, err := launchWorker(ctx, jobRoot, job)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		worker.abort()
		return ErrCancelled
	}
	m.mu.Lock()
	m.worker = worker
	m.state.Phase = "restarting"
	m.mu.Unlock()
	return nil
}

func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil && m.state.Phase != "restarting" {
		m.cancel()
	}
}

func (m *Manager) Abort(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.worker != nil {
		m.worker.abort()
		m.worker = nil
	}
	m.state.Phase = "failed"
	m.state.Error = err.Error()
}

func (m *Manager) Commit() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.worker == nil {
		return ErrRestart
	}
	return m.worker.commit()
}

// AcknowledgeRestart runs only from the main instance's InitializeClient, after
// the native window is ready and startup configuration checks have passed.
// It reports a confirmed update restart so the caller can restore the window.
func (m *Manager) AcknowledgeRestart() bool {
	if len(os.Args) != 3 || os.Args[1] != "--jeemi-update-restarted" || !idPattern.MatchString(os.Args[2]) {
		return false
	}
	jobRoot := filepath.Join(m.root, os.Args[2])
	var job installJob
	data, err := readBounded(filepath.Join(jobRoot, "job.json"), 16384)
	if err != nil || json.Unmarshal(data, &job) != nil || job.ID != os.Args[2] {
		return false
	}
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil || executable != job.Executable {
		return false
	}
	if err = os.WriteFile(filepath.Join(jobRoot, "started"), []byte(m.current), 0600); err != nil {
		return false
	}
	m.mu.Lock()
	m.state = State{Phase: "idle", Version: job.Version, JobID: job.ID}
	m.mu.Unlock()
	return true
}

// Consuming a displayed receipt is separate from the read-only state API.
func (m *Manager) DismissResult(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !idPattern.MatchString(id) || busy(m.state.Phase) {
		return
	}
	var receipt State
	data, err := readBounded(filepath.Join(m.root, "result.json"), 4096)
	if err != nil || json.Unmarshal(data, &receipt) != nil || receipt.JobID != id {
		return
	}
	if receipt.Phase != "succeeded" && receipt.Phase != "failed" {
		return
	}
	receipt.Phase = "acknowledged"
	writeReceipt(m.root, receipt)
	if m.state.JobID == id {
		m.state = State{Phase: "idle", Version: m.state.Version}
	}
}

func (m *Manager) cleanupPreviousJob() {
	var receipt State
	filename := filepath.Join(m.root, "result.json")
	data, err := readBounded(filename, 4096)
	if err != nil || json.Unmarshal(data, &receipt) != nil || !idPattern.MatchString(receipt.JobID) {
		return
	}
	if receipt.Phase != "succeeded" && receipt.Phase != "failed" && receipt.Phase != "acknowledged" {
		return
	}
	info, err := os.Stat(filename)
	if err != nil || time.Since(info.ModTime()) < 5*time.Second {
		return
	}
	// Only the exact completed job is eligible, never arbitrary cache folders.
	_ = os.RemoveAll(filepath.Join(m.root, receipt.JobID))
}
