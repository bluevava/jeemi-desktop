package externalui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type State struct {
	Installed          []Release `json:"installed"`
	Available          []Release `json:"available"`
	LatestVersion      string    `json:"latestVersion"`
	LastCheckedAt      string    `json:"lastCheckedAt"`
	Phase              string    `json:"phase"`
	DownloadingVersion string    `json:"downloadingVersion"`
	ReceivedBytes      int64     `json:"receivedBytes"`
	TotalBytes         int64     `json:"totalBytes"`
	LastError          string    `json:"lastError"`
}

type Manager struct {
	operation sync.Mutex
	mu        sync.RWMutex
	root      string
	staging   string
	client    *http.Client
	api       string
	state     State
	cancel    context.CancelFunc
}

type Options struct {
	DataDirectory string
	HTTPClient    *http.Client
}

func NewManager(options Options) *Manager {
	dataRoot := options.DataDirectory
	manager := &Manager{
		root:    filepath.Join(dataRoot, "runtime", "mihomo", filepath.FromSlash(RuntimeDirectory)),
		staging: filepath.Join(dataRoot, "cache", "zashboard", "staging"),
		api:     officialAPI,
		client: &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > 10 || request.URL.Scheme != "https" {
				return fmt.Errorf("unsupported zashboard redirect")
			}
			return nil
		}},
	}
	if options.HTTPClient != nil {
		manager.client = options.HTTPClient
	}
	return manager
}

// State is read-only and offline, including when no cache exists yet.
func (m *Manager) State() State {
	m.mu.RLock()
	state := m.state
	state.Available = append([]Release{}, state.Available...)
	m.mu.RUnlock()
	state.Installed = []Release{}
	entries, _ := os.ReadDir(m.root)
	for _, entry := range entries {
		if entry.IsDir() && ValidVersion(entry.Name()) {
			if release, err := m.installed(entry.Name(), false); err == nil {
				state.Installed = append(state.Installed, release)
			}
		}
	}
	sort.Slice(state.Installed, func(i, j int) bool { return Newer(state.Installed[i].Version, state.Installed[j].Version) })
	return state
}

func (m *Manager) begin(ctx context.Context, phase, version string) (context.Context, func(error), error) {
	if !m.operation.TryLock() {
		return nil, nil, fmt.Errorf("zashboard operation already in progress")
	}
	ctx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancel = cancel
	m.state.Phase, m.state.DownloadingVersion, m.state.LastError = phase, version, ""
	m.state.ReceivedBytes, m.state.TotalBytes = 0, 0
	m.mu.Unlock()
	return ctx, func(err error) {
		cancelled := err != nil && ctx.Err() != nil
		cancel()
		m.mu.Lock()
		m.cancel = nil
		m.state.Phase, m.state.DownloadingVersion = "", ""
		if err != nil {
			m.state.LastError = "failed"
			if cancelled {
				m.state.LastError = "cancelled"
			}
		}
		m.mu.Unlock()
		m.operation.Unlock()
	}, nil
}

func (m *Manager) Cancel() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) Check(ctx context.Context) (err error) {
	ctx, done, err := m.begin(ctx, "checking", "")
	if err != nil {
		return err
	}
	defer func() { done(err) }()
	releases, err := m.fetch(ctx, "?per_page=30")
	if err != nil {
		return err
	}
	latest, err := m.fetch(ctx, "/latest")
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.state.Available = releases
	m.state.LatestVersion = latest[0].Version
	m.state.LastCheckedAt = time.Now().UTC().Format(time.RFC3339)
	m.mu.Unlock()
	return nil
}

// Ensure downloads only missing resources. An empty selection first reuses the
// newest installed release; it contacts GitHub only if there is no local UI.
func (m *Manager) Ensure(ctx context.Context, version string) (string, error) {
	if version == "" {
		installed := m.State().Installed
		if len(installed) > 0 {
			version = installed[0].Version
		}
	}
	if version != "" {
		if _, err := m.Installed(version); err == nil {
			return version, nil
		}
	}
	return m.Download(ctx, version)
}

// Download accepts a release tag, or the empty string for GitHub's latest
// formal release. Existing valid versions are never downloaded again.
func (m *Manager) Download(ctx context.Context, version string) (selected string, err error) {
	if version != "" && !ValidVersion(version) {
		return "", fmt.Errorf("invalid zashboard version")
	}
	if version != "" {
		if _, err := m.Installed(version); err == nil {
			return version, nil
		}
	}
	ctx, done, err := m.begin(ctx, "downloading", version)
	if err != nil {
		return "", err
	}
	defer func() { done(err) }()
	suffix := "/latest"
	if version != "" {
		suffix = "/tags/" + version
	}
	releases, err := m.fetch(ctx, suffix)
	if err != nil {
		return "", err
	}
	release := releases[0]
	if version != "" && release.Version != version {
		return "", fmt.Errorf("zashboard release tag mismatch")
	}
	if _, err := m.Installed(release.Version); err == nil {
		return release.Version, nil
	}
	m.mu.Lock()
	m.state.DownloadingVersion, m.state.TotalBytes = release.Version, release.Size
	m.mu.Unlock()
	if err := m.install(ctx, release); err != nil {
		return "", err
	}
	return release.Version, nil
}
